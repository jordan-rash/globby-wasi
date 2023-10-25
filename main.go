package main

import (
	"fmt"
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

	if trimmedPath[0] == "" || trimmedPath[0] == "/" {
		writeHttpResponse(resp, http.StatusNotFound, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\"please provide a file name -> 'example.com/myfile.txt'\"}"))
		return
	}

	container := globby.WasiBlobstoreBlobstoreGetContainer(DEFAULT_CONTAINER_NAME)
	if container.IsErr() {
		globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelDebug(), "create_container", DEFAULT_CONTAINER_NAME+" did not exist -> creating")
		container = globby.WasiBlobstoreBlobstoreCreateContainer(DEFAULT_CONTAINER_NAME)
		if container.IsErr() {
			writeHttpResponse(resp, http.StatusNotFound, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\""+path+":"+container.UnwrapErr()+"}"))
			return
		}
	}

	switch {
	case method == globby.WasiHttpTypesMethodGet():
		globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelDebug(), "Handle", "inside GET")
		exists := globby.WasiBlobstoreContainerHasObject(container.Unwrap(), trimmedPath[0])
		if exists.IsErr() {
			writeHttpResponse(resp, http.StatusInternalServerError, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\""+path+":"+exists.UnwrapErr()+"}"))
			return
		}

		if !exists.Val {
			writeHttpResponse(resp, http.StatusNotFound, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"msg\":\"file does not exist\"}"))
			return
		}

		globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelDebug(), "Handle", "creating blobstore stream")
		dataStream := globby.WasiBlobstoreContainerGetData(container.Unwrap(), trimmedPath[0], 0, 18446744073709551614)
		if dataStream.IsErr() {
			writeHttpResponse(resp, http.StatusInternalServerError, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\""+path+":"+dataStream.UnwrapErr()+"}"))
			return
		}

		globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelDebug(), "Handle", "reading blobstore stream")
		data := globby.WasiBlobstoreTypesIncomingValueConsumeSync(dataStream.Unwrap())
		if data.IsErr() {
			writeHttpResponse(resp, http.StatusInternalServerError, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\""+path+":"+data.UnwrapErr()+"}"))
			return
		}

		globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelDebug(), "Handle", "writing data")
		writeHttpResponse(resp, http.StatusOK, []globby.WasiHttpTypesTuple2StringListU8TT{}, data.Unwrap())
	case method == globby.WasiHttpTypesMethodPost():
		globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelDebug(), "Handle", "inside POST")

		bodyStream := globby.WasiHttpTypesIncomingRequestConsume(req) // incoming-body
		if bodyStream.IsErr() {
			writeHttpResponse(resp, http.StatusBadRequest, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\""+path+":\"bad payload\"}"))
			return
		}

		globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelDebug(), "Handle", "1")

		var data []byte
		readStream := globby.WasiIoStreamsBlockingRead(bodyStream.Val, 18446744073709551614)
		for readStream.Val.F1 == globby.WasiIoStreamsStreamStatusOpen() {
			data = append(data, readStream.Val.F0...)
			readStream = globby.WasiIoStreamsBlockingRead(bodyStream.Val, 18446744073709551614)
		}

		outgoingValue := globby.WasiBlobstoreTypesNewOutgoingValue()
		outStream := globby.WasiBlobstoreTypesOutgoingValueWriteBody(outgoingValue)

		// -------------------------------------------------------------------------------------------

		pollable := globby.WasiIoStreamsSubscribeToOutputStream(outStream.Val)

		bIndex := 0
		for bIndex < len(data) {
			if globby.WasiPollPollPollOneoff([]uint32{pollable})[0] {
				globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelDebug(), "Handle", fmt.Sprintf("inside loop - bIndex: %d", bIndex))

				cw := globby.WasiIoStreamsCheckWrite(outStream.Val)
				if cw.IsErr() {
					return
				}

				globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelDebug(), "Handle", fmt.Sprintf("inside loop - checkWrite: %d", cw.Val))

				if int(cw.Val) > len(data) {
					cw.Val = uint64(len(data))
				}

				globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelDebug(), "Handle", fmt.Sprintf("inside loop - writing: %d-%d", bIndex, cw.Val))
				w := globby.WasiIoStreamsWrite(outStream.Val, data[bIndex:int(cw.Val)])
				if w.IsErr() {
					globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelError(), "Handle", fmt.Sprintf("failed to write to stream: %v", w.UnwrapErr()))
					return
				}

				bIndex = int(cw.Val) + 1
			}
		}

		f := globby.WasiIoStreamsFlush(outStream.Val)
		if f.IsErr() {
			globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelError(), "Handle", fmt.Sprintf("failed to flush to stream: %v", f.UnwrapErr()))
			return
		}

		// NOTE: I dont know why we have to do these two steps
		globby.WasiPollPollPollOneoff([]uint32{pollable})
		globby.WasiIoStreamsCheckWrite(outStream.Val)

		// -------------------------------------------------------------------------------------------

		wd := globby.WasiBlobstoreContainerWriteData(container.Unwrap(), trimmedPath[0], outgoingValue)
		if wd.IsErr() {
			writeHttpResponse(resp, http.StatusInternalServerError, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"error\":\""+path+":"+wd.UnwrapErr()+"}"))
			return
		}

		writeHttpResponse(resp, http.StatusOK, []globby.WasiHttpTypesTuple2StringListU8TT{{F0: "Content-Type", F1: []byte("application/json")}}, []byte("{\"msg\":\""+path+": uploaded\"}"))
	case method == globby.WasiHttpTypesMethodDelete():

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
	globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelDebug(), "writeHttpResponse", fmt.Sprintf("writing response: len=%d", len(body)))

	headers := globby.WasiHttpTypesNewFields(inHeaders)

	outgoingResponse := globby.WasiHttpTypesNewOutgoingResponse(statusCode, headers)
	if outgoingResponse.IsErr() {
		return
	}

	outgoingStream := globby.WasiHttpTypesOutgoingResponseWrite(outgoingResponse.Unwrap())
	if outgoingStream.IsErr() {
		return
	}

	pollable := globby.WasiIoStreamsSubscribeToOutputStream(outgoingStream.Val)

	bIndex := 0
	for bIndex < len(body) {
		if globby.WasiPollPollPollOneoff([]uint32{pollable})[0] {
			globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelDebug(), "writeHttpResponse", fmt.Sprintf("inside loop - bIndex: %d", bIndex))

			cw := globby.WasiIoStreamsCheckWrite(outgoingStream.Val)
			if cw.IsErr() {
				return
			}

			globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelDebug(), "writeHttpResponse", fmt.Sprintf("inside loop - checkWrite: %d", cw.Val))

			if int(cw.Val) > len(body) {
				cw.Val = uint64(len(body))
			}

			globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelDebug(), "writeHttpResponse", fmt.Sprintf("inside loop - writing: %d-%d", bIndex, cw.Val))
			w := globby.WasiIoStreamsWrite(outgoingStream.Val, body[bIndex:int(cw.Val)])
			if w.IsErr() {
				globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelError(), "writeHttpResponse", fmt.Sprintf("failed to write to stream: %v", w.UnwrapErr()))
				return
			}

			bIndex = int(cw.Val) + 1
		}
	}

	f := globby.WasiIoStreamsFlush(outgoingStream.Val)
	if f.IsErr() {
		globby.WasiLoggingLoggingLog(globby.WasiLoggingLoggingLevelError(), "writeHttpResponse", fmt.Sprintf("failed to flush to stream: %v", f.UnwrapErr()))
		return
	}

	globby.WasiHttpTypesFinishOutgoingStream(outgoingStream.Val)

	// NOTE: I dont know why we have to do these two steps
	globby.WasiPollPollPollOneoff([]uint32{pollable})
	globby.WasiIoStreamsCheckWrite(outgoingStream.Val)

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
