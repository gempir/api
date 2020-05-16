package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"gopkg.in/h2non/bimg.v1"
)

var (
	supportedThumbnails = []string{"image/jpeg", "image/png", "image/gif"}
)

const (
	// max width or height the thumbnail will be resized to
	maxThumbnailSize = 300
)

func doThumbnailRequest(urlString string, r *http.Request) (interface{}, error, time.Duration) {
	url, err := url.Parse(urlString)
	if err != nil {
		return rInvalidURL, nil, noSpecialDur
	}

	resp, err := makeRequest(url.String())
	if err != nil {
		if strings.HasSuffix(err.Error(), "no such host") {
			return rNoLinkInfoFound, nil, noSpecialDur
		}

		return marshalNoDur(&LinkResolverResponse{
			Status:  http.StatusInternalServerError,
			Message: clean(err.Error()),
		})
	}

	defer resp.Body.Close()

	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		contentLengthBytes, err := strconv.Atoi(contentLength)
		if err != nil {
			return nil, err, noSpecialDur
		}
		if contentLengthBytes > maxContentLength {
			return rResponseTooLarge, nil, noSpecialDur
		}
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusMultipleChoices {
		fmt.Println("Skipping url", resp.Request.URL, "because status code is", resp.StatusCode)
		return rNoLinkInfoFound, nil, noSpecialDur
	}

	if !isSupportedThumbnail(resp.Header.Get("content-type")) {
		return rNoLinkInfoFound, nil, noSpecialDur
	}

	image, err := buildResizedThumbnail(resp)
	if err != nil {
		log.Println(err.Error())
		return rNoLinkInfoFound, nil, noSpecialDur
	}

	return image, nil, 10 * time.Minute
}

func isSupportedThumbnail(contentType string) bool {
	for _, supportedType := range supportedThumbnails {
		if contentType == supportedType {
			return true
		}
	}

	return false
}

func thumbnail(w http.ResponseWriter, r *http.Request) {
	url, err := unescapeURLArgument(r, "url")
	if err != nil {
		_, err = w.Write(rInvalidURL)
		if err != nil {
			log.Println("Error writing response:", err)
		}
		return
	}

	response := thumbnailCache.Get(url, r)

	_, err = w.Write(response.([]byte))
	if err != nil {
		log.Println("Error writing response:", err)
	}
}

var thumbnailCache *loadingCache

func init() {
	thumbnailCache = newLoadingCache("thumbnail", doThumbnailRequest, 10*time.Minute)
}

func handleThumbnail(router *mux.Router) {
	router.HandleFunc("/thumbnail/{url:.*}", thumbnail).Methods("GET")
}

func buildResizedThumbnail(resp *http.Response) ([]byte, error) {
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, fmt.Errorf("Could not deocde image from url: %s", resp.Request.URL)
	}

	converted, err := bimg.NewImage(body).Convert(bimg.WEBP)
	if err != nil {
		return []byte{}, fmt.Errorf("Could not convert image from url: %s, error: %s", resp.Request.URL, err.Error())
	}

	resized, err := bimg.NewImage(converted).Resize(maxThumbnailSize, maxThumbnailSize)
	if err != nil {
		return []byte{}, fmt.Errorf("Could not resize image from url: %s, error: %s", resp.Request.URL, err.Error())
	}

	return resized, nil
}
