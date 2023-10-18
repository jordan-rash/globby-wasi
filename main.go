package main

import (
	globby "globby/gen"
	"net/http"
	"strings"
)

const (
	DEFAULT_CONTAINER_NAME string = "default"
	CONTAINER_HEADER_NAME  string = "blobby-container"
	CONTAINER_PARAM_NAME   string = "container"
)

type Globby struct{}

func (g *Globby) Handle(req globby.WasiHttpIncomingHandlerIncomingRequest, resp globby.WasiHttpTypesResponseOutparam) {
	method := globby.WasiHttpTypesIncomingRequestMethod(req)

	pathWithQuery := globby.WasiHttpTypesIncomingRequestPathWithQuery(req)
	if pathWithQuery.IsNone() {
		return
	}

	splitPathQuery := strings.Split(pathWithQuery.Unwrap(), "?")

	path := splitPathQuery[0]
	trimmedPath := strings.Split(strings.TrimPrefix(path, "/"), "/")
	switch {
	case method == globby.WasiHttpTypesMethodGet():
		container := globby.WasiBlobstoreBlobstoreGetContainer(DEFAULT_CONTAINER_NAME)
		if container.IsErr() {
			writeHttpResponse(resp, http.StatusNotFound, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\""+path+":"+container.UnwrapErr()+"}"))
			return
		}

		exists := globby.WasiBlobstoreContainerHasObject(container.Unwrap(), trimmedPath[0])
		if exists.IsErr() {
			writeHttpResponse(resp, http.StatusNoContent, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\""+path+":"+exists.UnwrapErr()+"}"))
			return
		}

		dataStream := globby.WasiBlobstoreContainerGetData(container.Unwrap(), trimmedPath[0], 0, 18446744073709551614)
		if dataStream.IsErr() {
			writeHttpResponse(resp, http.StatusInternalServerError, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\""+path+":"+dataStream.UnwrapErr()+"}"))
			return
		}

		data := globby.WasiBlobstoreTypesIncomingValueConsumeSync(dataStream.Unwrap())
		if data.IsErr() {
			writeHttpResponse(resp, http.StatusInternalServerError, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\""+path+":"+data.UnwrapErr()+"}"))
			return
		}

		writeHttpResponse(resp, http.StatusOK, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/text")}}, data.Unwrap())
	case method == globby.WasiHttpTypesMethodPost():
		container := globby.WasiBlobstoreBlobstoreGetContainer(DEFAULT_CONTAINER_NAME)
		if container.IsErr() {
			writeHttpResponse(resp, http.StatusNotFound, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\""+path+":"+container.UnwrapErr()+"}"))
			return
		}

		bodyStream := globby.WasiHttpTypesIncomingRequestConsume(req) // incoming-body
		if bodyStream.IsErr() {
			writeHttpResponse(resp, http.StatusBadRequest, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\""+path+":\"bad payload\"}"))
			return
		}

		readStream := globby.WasiIoStreamsBlockingRead(bodyStream.Val, 18446744073709551614)
		globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelInfo(), "STREAM DATA", string(readStream.Val.F0))

		outgoingValue := globby.WasiBlobstoreTypesNewOutgoingValue()
		outStream := globby.WasiBlobstoreTypesOutgoingValueWriteBody(outgoingValue)

		cw := globby.WasiIoStreamsCheckWrite(outStream.Val)
		if cw.IsErr() {
			writeHttpResponse(resp, http.StatusInternalServerError, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\"failed check write\"}"))
			return
		}

		wf := globby.WasiIoStreamsBlockingWriteAndFlush(outStream.Val, readStream.Val.F0)
		if wf.IsErr() {
			return
		}

		wd := globby.WasiBlobstoreContainerWriteData(container.Unwrap(), trimmedPath[0], outgoingValue)
		if wd.IsErr() {
			writeHttpResponse(resp, http.StatusInternalServerError, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\""+path+":"+wd.UnwrapErr()+"}"))
			return
		}

		writeHttpResponse(resp, http.StatusOK, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"msg\":\""+path+": uploaded\"}"))
	case method == globby.WasiHttpTypesMethodDelete():
		container := globby.WasiBlobstoreBlobstoreGetContainer(DEFAULT_CONTAINER_NAME)
		if container.IsErr() {
			writeHttpResponse(resp, http.StatusNotFound, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\""+path+":"+container.UnwrapErr()+"}"))
			return
		}

		del := globby.WasiBlobstoreContainerDeleteObject(container.Val, trimmedPath[0])
		if del.IsErr() {
			writeHttpResponse(resp, http.StatusNotFound, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\""+path+":"+del.UnwrapErr()+"}"))
			return
		}

		writeHttpResponse(resp, http.StatusOK, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"msg\":\""+path+": deleted\"}"))
	default:
		writeHttpResponse(resp, http.StatusMethodNotAllowed, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\""+path+": method not allowed\"}"))
		return
	}

}

func writeHttpResponse(responseOutparam globby.WasiHttpTypesResponseOutparam, statusCode uint16, inHeaders []globby.WasiHttpTypesTuple2StringListU8TT, body []byte) {
	headers := globby.WasiHttpTypesNewFields(inHeaders)

	outgoingResponse := globby.WasiHttpTypesNewOutgoingResponse(statusCode, headers)
	if outgoingResponse.IsErr() {
		return
	}

	outgoingStream := globby.WasiHttpTypesOutgoingResponseWrite(outgoingResponse.Unwrap())
	if outgoingStream.IsErr() {
		return
	}

	cw := globby.WasiIoStreamsCheckWrite(outgoingStream.Val)
	if cw.IsErr() {
		return
	}

	wf := globby.WasiIoStreamsBlockingWriteAndFlush(outgoingStream.Val, body)
	if wf.IsErr() {
		return
	}

	globby.WasiHttpTypesFinishOutgoingStream(outgoingStream.Val)

	outparm := globby.WasiHttpTypesSetResponseOutparam(responseOutparam, outgoingResponse)
	if outparm.IsErr() {
		return
	}
}

func init() {
	mg := new(Globby)
	globby.SetExportsWasiHttpIncomingHandler(mg)
}

//go:generate wit-bindgen tiny-go ./wit -w globby --out-dir=gen
func main() {}
