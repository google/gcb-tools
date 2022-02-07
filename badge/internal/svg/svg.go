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
	"github.com/golang/freetype/truetype"
	"github.com/gorilla/mux"
	cloudbuildpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
)

// Metadata used for rendering the SVG template.
type SvgInfo struct {
	// Total height and width of the badge
	Height int
	Width  int

	// Text to use for the label
	LabelText string
	// X position offset to start displaying label text
	LabelStart int
	// Total width of the label
	LabelWidth int
	// Color to use for the label background
	LabelColor string

	// Corresponding Message values
	MessageText  string
	MessageColor string
	MessageStart int
	MessageWidth int

	// Font-size to use
	FontSize int
	// Y coordinate to draw the text in the label and message
	TextY int
}

// Creates a SvgInfo with the pre-defined and derived values for a cloud build
// build status, projectId name, using the given font.
// We pass in the font so it can be shared and not recreated on each request.
func createSvgInfo(status cloudbuildpb.Build_Status, projectId string, font *truetype.Font) (*SvgInfo, error) {
	info := &SvgInfo{
		LabelColor:   "#555",
		LabelText:    projectId,
		LabelStart:   6,
		MessageText:  status.String(),
		MessageColor: "#f00",
		Width:        100,
		Height:       20,
		FontSize:     12,
		TextY:        16,
	}
	switch status {
	case cloudbuildpb.Build_PENDING, cloudbuildpb.Build_QUEUED, cloudbuildpb.Build_WORKING:
		info.MessageColor = "#aaa"
	case cloudbuildpb.Build_SUCCESS:
		info.MessageColor = "#97ca00"
	}

	// Create a context so we can draw and measure the font metrics.
	// Alternatively we could grab the height directly and iterate over
	// the string to have the correct kerning between the runes.
	ctx := freetype.NewContext()
	ctx.SetFont(font)
	ctx.SetFontSize(float64(info.FontSize))

	// Since we can't get this from the context, build up the right scale
	// so we can query for font metrics.
	dpi := 72.0
	scale := fixed.Int26_6(float64(info.FontSize) * dpi * (64.0 / 72.0))
	vmetric := font.VMetric(scale, 69) // 69 is the letter 'E'
	// Place the bottom of the text at the line spacing height
	info.TextY = vmetric.AdvanceHeight.Ceil()

	// Draw the label to see the width.  Add in the front and middle
	// padding (of 6 and 4 respectively)
	pair, err := ctx.DrawString(info.LabelText, fixed.Point26_6{})
	if err != nil {
		return nil, err
	}
	info.LabelWidth = pair.X.Ceil() + 6 + 4

	// Draw the message to see the width.  Add in the front and middle
	// padding (of 6 and 4 respectively)
	pair, err = ctx.DrawString(info.MessageText, fixed.Point26_6{})
	if err != nil {
		return nil, err
	}
	info.MessageWidth = pair.X.Ceil() + 6 + 4
	// Start the message after the 4px padding
	info.MessageStart = info.LabelWidth + 4
	// Set width now that we know the right sizes.
	info.Width = info.LabelWidth + info.MessageWidth
	// Add in the top and bottom padding of 4px each.
	info.Height = vmetric.AdvanceHeight.Ceil() + 4 + 4

	return info, nil
}

func http500(err error) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("500: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func loadFont(fontfile string) (*truetype.Font, error) {
	fmt.Printf("Loading fontfile: '%q'\n", fontfile)
	b, err := ioutil.ReadFile(fontfile)
	if err != nil {
		return nil, err
	}
	return freetype.ParseFont(b)
}

//func createTemplate

func Handle(ctx context.Context, c *cloudbuild.Client, fontfile string) func(http.ResponseWriter, *http.Request) {
	// N.B. Font weight 400 is regular, 700 is bold.
	templateText := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{{.Width}}" height="{{.Height}}">
    <style>
        @import url("https://fonts.googleapis.com/css?family=Open+Sans:400");
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
        <text x="{{.LabelStart}}" y="{{.TextY}}" font-family="Open Sans" font-size="{{.FontSize}}" fill="#fff">{{.LabelText}}</text>
        <rect fill="{{.MessageColor}}" x="{{.LabelWidth}}" y="0" width="{{.Width}}" height="{{.Height}}"/>
        <text x="{{.MessageStart}}" y="{{.TextY}}" font-family="Open Sans" font-size="{{.FontSize}}" fill="#fff">{{.MessageText}}</text>
        <rect fill="url(,1)" x="0" y="0" width="{{.Width}}" height="{{.Height}}"/>
    </g>
    <g fill="#eee">
        <use x="0" y="1" fill="#010101" fill-opacity=".3" xlink:href="#text"/>
        <use x="0" y="0" xlink:href="#text"/>
    </g>
</svg>`

	tmpl, err := template.New("badge").Parse(templateText)
	if err != nil {
		return http500(err)
	}

	font, err := loadFont(fontfile)
	if err != nil {
		return http500(err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		projectId := params["projectId"]
		build, err := client.GetLastBuild(ctx, c, projectId)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		info, err := createSvgInfo(build.Status, projectId, font)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		w.Header().Set("content-type", "image/svg+xml")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		date := time.Now().Format(http.TimeFormat)
		w.Header().Set("Date", date)
		w.Header().Set("Expires", date)
		if build.Id != "" {
			// Since build ID is unique and can only change with the status, this seems
			// like a wonderful etag if the client pays attention.
			w.Header().Set("ETag", build.Id+build.Status.String())
		}

		err = tmpl.Execute(w, info)
		if err != nil {
			log.Printf("500: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
}
