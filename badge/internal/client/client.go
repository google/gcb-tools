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
	"fmt"
	"log"

	cloudbuild "cloud.google.com/go/cloudbuild/apiv1"
	"google.golang.org/api/iterator"
	cloudbuildpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
)

func GetLastBuild(ctx context.Context, c *cloudbuild.Client, projectId string) (*cloudbuildpb.Build, error) {
	req := &cloudbuildpb.ListBuildsRequest{
		ProjectId: projectId,
	}
	it := c.ListBuilds(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			err := fmt.Errorf("build '%s' not found", projectId)
			return nil, err
		}
		if err != nil {
			return nil, err
		}

		log.Printf("Build: %v/%v", resp.ProjectId, resp.Id)
		if resp.ProjectId == projectId {
			return resp, nil
		}
	}
}
