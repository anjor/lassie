package httpserver

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	lassie "github.com/filecoin-project/lassie/pkg/lassie"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync/storeutil"
	"github.com/ipld/go-car/v2/blockstore"
)

func ipfsHandler(lassie *lassie.Lassie, writer io.Writer) func(http.ResponseWriter, *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		printer := NewHandlerPrinterWithWriter(writer)
		printer.PrintPath(req)

		urlPath := strings.Split(req.URL.Path, "/")[1:]

		// filter out everything but GET requests
		switch req.Method {
		case http.MethodGet:
			break
		default:
			printer.PrintStatus(req, http.StatusMethodNotAllowed)
			res.Header().Add("Allow", http.MethodGet)
			res.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// check if cid path param is missing
		if len(urlPath) < 2 {
			// not a valid path to hit
			printer.PrintStatus(req, http.StatusNotFound)
			res.WriteHeader(http.StatusNotFound)
			return
		}

		// check if Accept header includes application/vnd.ipld.car
		hasAccept := req.Header.Get("Accept") != ""
		acceptTypes := strings.Split(req.Header.Get("Accept"), ",")
		validAccept := false
		for _, acceptType := range acceptTypes {
			typeParts := strings.Split(acceptType, ";")
			if typeParts[0] == "*/*" || typeParts[0] == "application/*" || typeParts[0] == "application/vnd.ipld.car" {
				validAccept = true
				break
			}
		}
		if hasAccept && !validAccept {
			printer.PrintStatusMessage(req, http.StatusBadRequest, "No acceptable content type")
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		// check if format is car
		hasFormat := req.URL.Query().Has("format")
		if hasFormat && req.URL.Query().Get("format") != "car" {
			printer.PrintStatusMessage(req, http.StatusBadRequest, fmt.Sprintf("Requested non-supported format %s", req.URL.Query().Get("format")))
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		// if neither are provided return
		// one of them has to be given with a car type since we only return car files
		if !validAccept && !hasFormat {
			printer.PrintStatusMessage(req, http.StatusBadRequest, "Neither a valid accept header or format parameter were provided")
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		// check if provided filename query parameter has .car extension
		if req.URL.Query().Has("filename") {
			filename := req.URL.Query().Get("filename")
			ext := filepath.Ext(filename)
			if ext == "" {
				printer.PrintStatusMessage(req, http.StatusBadRequest, "Filename missing extension")
				res.WriteHeader(http.StatusBadRequest)
				return
			}
			if ext != ".car" {
				printer.PrintStatusMessage(req, http.StatusBadRequest, fmt.Sprintf("Filename uses non-supported extension %s", ext))
				res.WriteHeader(http.StatusBadRequest)
				return
			}
		}

		// validate cid path parameter
		cidStr := urlPath[1]
		rootCid, err := cid.Parse(cidStr)
		if err != nil {
			printer.PrintStatusMessage(req, http.StatusInternalServerError, "Failed to parse cid path parameter")
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Grab unixfs path if it exists
		// var unixfsPath []string
		// if len(urlPath) > 2 {
		// 	unixfsPath = urlPath[2:]
		// }
		// TODO: Do something with unixfs path

		log.Debug("creating temp car file")
		carfile, err := os.CreateTemp("", rootCid.String())
		if err != nil {
			printer.PrintStatusMessage(req, http.StatusInternalServerError, fmt.Sprintf("Failed to create temp car file before retrieval: %v", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		// create blockstore from defined car file and use it for the link system
		blockstore, err := blockstore.OpenReadWriteFile(carfile, []cid.Cid{rootCid})
		if err != nil {
			printer.PrintStatusMessage(req, http.StatusInternalServerError, fmt.Sprintf("Failed to create blockstore from temp car file before retrieval: %v", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		linkSystem := storeutil.LinkSystemForBlockstore(blockstore)

		log.Debugw("fetching cid into car file", "cid", rootCid.String(), "file", carfile.Name())
		id, _, err := lassie.Fetch(req.Context(), rootCid, linkSystem)
		if err != nil {
			printer.PrintStatusMessage(req, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch cid: %v", err))
			res.WriteHeader(http.StatusInternalServerError)
			carfile.Close()
			return
		}

		log.Debugw("closing car file", "file", carfile.Name())
		err = carfile.Close()
		if err != nil {
			printer.PrintStatusMessage(req, http.StatusInternalServerError, fmt.Sprintf("Failed to close temp car file after retrieval: %v", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		// set Content-Disposition header based on filename url parameter
		var filename string
		if req.URL.Query().Has("filename") {
			filename = req.URL.Query().Get("filename")
		} else {
			filename = fmt.Sprintf("%s.car", rootCid.String())
		}
		res.Header().Set("Content-Disposition", "attachment; filename="+filename)

		res.Header().Set("Accept-Ranges", "none")
		res.Header().Set("Cache-Control", "public, max-age=29030400, immutable")
		res.Header().Set("Content-Type", "application/vnd.ipld.car; version=1")
		res.Header().Set("Etag", fmt.Sprintf("%s.car", rootCid.String()))
		res.Header().Set("C-Content-Type-Options", "nosniff")
		res.Header().Set("X-Ipfs-Path", req.URL.Path)
		// TODO: set X-Ipfs-Roots header
		res.Header().Set("X-Trace-Id", id.String())

		printer.PrintStatus(req, http.StatusOK)
		http.ServeFile(res, req, carfile.Name())

		log.Debugw("removing temp car file", "file", carfile.Name())
		err = os.Remove(carfile.Name())
		if err != nil {
			log.Errorw("failed to remove temp car file after retrieval", "file", carfile.Name(), "err", err)
		}
	}
}
