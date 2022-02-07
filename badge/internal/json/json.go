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
package json

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/googlestaging/gcb-tools/badge/internal/client"

	cloudbuild "cloud.google.com/go/cloudbuild/apiv1"
	"github.com/gorilla/mux"
	cloudbuildpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
)

// The JSON respresentation required by the Shields Endpoint
// See https://shields.io/endpoint
type ShieldEndpoint struct {
	SchemaVersion int    `json:"schemaVersion"`
	Label         string `json:"label"`
	Message       string `json:"message"`
	Color         string `json:"color"`
}

func createShieldEndpoint(status cloudbuildpb.Build_Status, projectId string) ShieldEndpoint {
	response := ShieldEndpoint{
		SchemaVersion: 1,
		Label:         projectId,
		Message:       status.String(),
		Color:         "red",
	}
	switch status {
	case cloudbuildpb.Build_PENDING, cloudbuildpb.Build_QUEUED, cloudbuildpb.Build_WORKING:
		response.Color = "grey"
	case cloudbuildpb.Build_SUCCESS:
		response.Color = "lightgreen"
	}
	return response
}

func Handle(ctx context.Context, c *cloudbuild.Client) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		projectId := params["projectId"]
		build, err := client.GetLastBuild(ctx, c, projectId)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		shield := createShieldEndpoint(build.Status, projectId)
		json.NewEncoder(w).Encode(shield)
	}
}
