/*
   Copyright 2022 Google LLC

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       https://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/
package svg

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"text/template"
	"time"

	"github.com/googlestaging/gcb-tools/badge/internal/client"
	"golang.org/x/image/math/fixed"

	cloudbuild "cloud.google.com/go/cloudbuild/apiv1"
	"github.com/golang/freetype"
	"github.com/gorilla/mux"
	cloudbuildpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
)

// Metadata used for rendering the SVG template.
type SvgInfo struct {
	Height       int
	Width        int
	LabelText    string
	LabelStart   int
	LabelWidth   int
	LabelColor   string
	MessageText  string
	MessageColor string
	MessageStart int
	MessageWidth int
}

func createSvgInfo(status cloudbuildpb.Build_Status, trigger string) SvgInfo {
	response := SvgInfo{
		LabelColor:   "#555",
		LabelText:    "Build: " + trigger,
		MessageText:  status.String(),
		MessageColor: "#f00",
		Width:        100,
		Height:       20,
	}
	switch status {
	case cloudbuildpb.Build_PENDING, cloudbuildpb.Build_QUEUED, cloudbuildpb.Build_WORKING:
		response.MessageColor = "#aaa"
	case cloudbuildpb.Build_SUCCESS:
		response.MessageColor = "#97ca00"
	}
	return response
}

func Handle(ctx context.Context, c *cloudbuild.Client, fontfile string) func(http.ResponseWriter, *http.Request) {
	// N.B. Font weight 400 is regular, 700 is bold.
	templateText := `
	<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{{.Width}}" height="{{.Height}}">
    <style>
        @import url("https://fonts.googleapis.com/css?family=Open+Sans:400,700");
    </style>
    <defs>
        <linearGradient id="glow" x2="0" y2="100%">
            <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
            <stop offset="1" stop-opacity=".1"/>
        </linearGradient>
        <mask id="mask">
            <rect width="{{.Width}}" height="{{.Height}}" rx="3" fill="#fff"/>
        </mask>
    </defs>

    <g mask="url(#mask)">
        <rect fill="{{.LabelColor}}" x="0" y="0" width="{{.LabelWidth}}" height="{{.Height}}"/>
        <text x="{{.LabelStart}}" y="14" font-family="Open Sans" font-size="12" fill="#fff">{{.LabelText}}</text>
        <rect fill="{{.MessageColor}}" x="{{.LabelWidth}}" y="0" width="{{.Width}}" height="{{.Height}}"/>
        <text x="{{.MessageStart}}" y="14" font-family="Open Sans" font-size="12" fill="#fff">{{.MessageText}}</text>
        <rect fill="url(,1)" x="0" y="0" width="{{.Width}}" height="{{.Height}}"/>
    </g>
    <g fill="#eee">
        <use x="0" y="1" fill="#010101" fill-opacity=".3" xlink:href="#text"/>
        <use x="0" y="0" xlink:href="#text"/>
    </g>
</svg>	
	`

	tmpl, err := template.New("badge").Parse(templateText)
	if err != nil {
		log.Printf("500: %v", err)
		return func(w http.ResponseWriter, r *http.Request) {
			log.Printf("500: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}

	fmt.Printf("Loading fontfile %q\n", fontfile)
	b, err := ioutil.ReadFile(fontfile)
	if err != nil {
		return func(w http.ResponseWriter, r *http.Request) {
			log.Printf("500: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}
	font, err := freetype.ParseFont(b)
	if err != nil {
		return func(w http.ResponseWriter, r *http.Request) {
			log.Printf("500: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		trigger := params["trigger"]
		build, err := client.GetLastBuild(ctx, c, trigger)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		info := createSvgInfo(build.Status, trigger)
		ctx := freetype.NewContext()
		ctx.SetFont(font)
		ctx.SetFontSize(12)

		pair, err := ctx.DrawString(info.LabelText, fixed.Point26_6{})
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		info.LabelWidth = pair.X.Ceil() + 10

		pair, err = ctx.DrawString(info.MessageText, fixed.Point26_6{})
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		info.LabelStart = 6
		info.MessageWidth = pair.X.Ceil() + 10
		info.MessageStart = info.LabelWidth + 4
		// Set width now that we know the right sizes.
		info.Width = info.LabelWidth + info.MessageWidth

		w.Header().Set("content-type", "image/svg+xml")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		date := time.Now().Format(http.TimeFormat)
		w.Header().Set("Date", date)
		w.Header().Set("Expires", date)
		if build.Id != "" {
			w.Header().Set("ETag", build.Id)
		}

		err = tmpl.Execute(w, info)
		if err != nil {
			log.Printf("500: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
}