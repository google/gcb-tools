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
	"net/http/httptest"
	"strings"

	"context"
	"net"
	"testing"

	"github.com/gorilla/mux"

	cloudbuild "cloud.google.com/go/cloudbuild/apiv1"
	"google.golang.org/api/option"
	"google.golang.org/grpc"

	cloudbuildpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
)

func TestSvgInfo(t *testing.T) {
	font, err := loadFont("../../third_party/googlefonts/opensans/fonts/ttf/OpenSans-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	info, err := createSvgInfo(cloudbuildpb.Build_SUCCESS, "hello", font)
	if err != nil {
		t.Fatal(err)
	}

	if info.Height != 25 {
		t.Fatalf("Height not 25, got %d", info.Height)
	}
}

type fakeCloudBuildServer struct {
	cloudbuildpb.UnimplementedCloudBuildServer
}

func (fake *fakeCloudBuildServer) ListBuilds(context.Context, *cloudbuildpb.ListBuildsRequest) (*cloudbuildpb.ListBuildsResponse, error) {
	response := &cloudbuildpb.ListBuildsResponse{
		Builds: []*cloudbuildpb.Build{{
			Name:           "",
			Id:             "abcdefg",
			ProjectId:      "project-success",
			Status:         cloudbuildpb.Build_SUCCESS,
			StatusDetail:   "",
			BuildTriggerId: "mytrigger",
		}, {
			Name:           "",
			Id:             "qwerty",
			ProjectId:      "project-failure",
			Status:         cloudbuildpb.Build_FAILURE,
			StatusDetail:   "",
			BuildTriggerId: "mytrigger2",
		}},
		NextPageToken: "",
	}
	return response, nil
}

func TestHandler(t *testing.T) {
	ctx := context.Background()
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	gsrv := grpc.NewServer()
	fakeCloudBuildServer := &fakeCloudBuildServer{}
	cloudbuildpb.RegisterCloudBuildServer(gsrv, fakeCloudBuildServer)
	addr := l.Addr().String()
	go func() {
		if err := gsrv.Serve(l); err != nil {
			panic(err)
		}
	}()

	// Create a client.
	c, err := cloudbuild.NewClient(ctx,
		option.WithEndpoint(addr),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithInsecure()),
	)
	if err != nil {
		t.Fatal(err)
	}

	f := Handle(ctx, c, "../../third_party/googlefonts/opensans/fonts/ttf/OpenSans-Regular.ttf")

	tables := []struct {
		url        string
		projectId  string
		code       int
		bodySubstr string
	}{
		{"http://localhost:8080/project-success/status.svg", "project-success", 200, "<svg xmlns=\"http://www.w3.org/2000/svg\" xmlns:xlink=\"http://www.w3.org/1999/xlink\" width=\"157\" height=\"25\">"},
		{"http://localhost:8080/project-failure/status.svg", "project-failure", 200, "<svg xmlns"},
		{"http://localhost:8080/project-foo/status.svg", "project-foo", 404, ""},
	}
	for _, table := range tables {
		r := httptest.NewRequest("GET", table.url, nil)
		w := httptest.NewRecorder()
		r = mux.SetURLVars(r, map[string]string{"projectId": table.projectId})
		f(w, r)

		if w.Code != table.code {
			t.Fatalf("%d expected, got %d", table.code, w.Code)
		}
		if table.bodySubstr != "" {
			if !strings.Contains(w.Body.String(), table.bodySubstr) {
				t.Fatalf("'%s' expected to be found in:\n %s", table.bodySubstr, w.Body.String())
			}
		}
	}

}
