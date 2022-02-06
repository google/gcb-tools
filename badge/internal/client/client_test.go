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
package client

import (
	"context"
	"log"
	"net"
	"testing"

	cloudbuild "cloud.google.com/go/cloudbuild/apiv1"
	"google.golang.org/api/option"
	cloudbuildpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
	"google.golang.org/grpc"
)

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

func TestClient(t *testing.T) {
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

	tables := []struct {
		projname string
		err      bool
		status   cloudbuildpb.Build_Status
	}{
		{"project-success", false, cloudbuildpb.Build_SUCCESS},
		{"project-failure", false, cloudbuildpb.Build_FAILURE},
		{"project-notfound", true, cloudbuildpb.Build_STATUS_UNKNOWN},
	}
	for _, table := range tables {
		build, err := GetLastBuild(ctx, c, table.projname)
		if table.err && err != nil {
			log.Printf("Caught expected error for '%s'", table.projname)
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if build.ProjectId != table.projname {
			t.Fatalf("Expected '%s'', got '%s'", table.projname, build.ProjectId)
		}
	}
}
