/*
 Copyright 2020 The Tekton Authors
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
     http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

type triggerErrorPayload struct {
	Error string `json:"errorMessage,omitempty"`
}

type urlToMap func(string) (map[string]interface{}, error)

const (
	rootBranchBodyKey    = "add_branch_body"
	branchBodyUrlKey     = "branches_url"
	branchBodyContentKey = "branch_body"
)

func main() {
	http.HandleFunc("/", makeAddBranchBodyHandler(getBranchBody))
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", 8080), nil))
}

func makeAddBranchBodyHandler(urlFetcherDecoder urlToMap) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		payload := []byte(`{}`)
		var err error

		// Get the payload
		if r.Body != nil {
			defer r.Body.Close()
			payload, err = ioutil.ReadAll(r.Body)
			if err != nil {
				log.Printf("failed to read request body: %w", err)
				marshalError(err, w)
				return
			}
			if len(payload) == 0 {
				bodyError := errors.New("empty body, cannot add a branch")
				log.Printf("No body received: %w", bodyError)
				marshalError(bodyError, w)
				return
			}
		} else {
			bodyError := errors.New("empty body, cannot add a branch")
			log.Printf("failed to read request body: %w", err)
			marshalError(bodyError, w)
			return
		}

		// Get the json body
		jsonBody, err := decodeBody(payload)
		if err != nil {
			log.Printf("failed to decode the body: %w", err)
			marshalError(err, w)
			return
		}
		// Get the URL from the body
		branchUrl, err := getBranchUrl(jsonBody)
		if err != nil {
			log.Printf("failed to extract the PR URL from the body: %w", err)
			marshalError(err, w)
			return
		}

		// Get the Response from the branchUrl
		res, err := urlFetch(branchUrl)

		// Get the PR Body from the URL
		branchBody, err := urlFetcherDecoder(branchUrl)
		if err != nil {
			log.Printf("failed to get the PR body: %w", err)
			marshalError(err, w)
			return
		}
		// Add the PR body to the original body
		jsonBody[rootBranchBodyKey].(map[string]interface{})[branchBodyContentKey] = branchBody

		// Marshal the body
		responseBytes, err := json.Marshal(jsonBody)
		if err != nil {
			log.Printf("failed marshal the response body: %w", err)
			marshalError(err, w)
			return
		}
		// Set all the original headers
		for k, values := range r.Header {
			for _, v := range values {
				w.Header().Add(k, v)
			}
		}
		// Write the response
		n, err := w.Write(responseBytes)
		if err != nil {
			log.Printf("Failed to write response. Bytes written: %d. Error: %q", n, err)
		}
	}
}

func marshalError(err error, w http.ResponseWriter) {
	if err != nil {
		triggerBody := triggerErrorPayload{
			Error: err.Error(),
		}
		tPayload, err := json.Marshal(triggerBody)
		if err != nil {
			log.Printf("Failed to marshal the trigger body. Error: %q", err)
			http.Error(w, "{}", http.StatusBadRequest)
			return
		}
		http.Error(w, string(tPayload[:]), http.StatusBadRequest)
	}
}

func decodeBody(body []byte) (map[string]interface{}, error) {
	var jsonMap map[string]interface{}
	err := json.Unmarshal(body, &jsonMap)
	if err != nil {
		return nil, err
	}
	return jsonMap, nil
}

func getBranchUrl(body map[string]interface{}) (string, error) {
	addBranchBody, ok := body[rootBranchBodyKey]
	if !ok {
		return "", errors.New("no 'add_branch_body' found in the body")
	}
	branchUrl, ok := addBranchBody.(map[string]interface{})[branchBodyUrlKey]
	if !ok {
		return "", errors.New("no 'body-url' found")
	}
	branchUrlString, ok := branchUrl.(string)
	if !ok {
		return "", errors.New("'body-url' found, but not a string")
	}
	return branchUrlString, nil
}

func getBranchBody(branchUrl string) (map[string]interface{}, error) {
	resp, err := http.Get(branchUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return decodeBody(body)
}

func addBranchBody(branchBody, originalBody map[string]interface{}) map[string]interface{} {
	return originalBody
}