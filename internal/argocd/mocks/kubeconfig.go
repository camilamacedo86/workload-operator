/*
Copyright 2023 Camila Macedo.

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

// Package mocks store mocks to be used in the tests under the internal directory
package mocks

// MockKubeConfig stores a mock for a KubeConfig
const MockKubeConfig = `
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: mocks
    server: https://your-cluster-server-here
  name: Test
contexts:
- context:
    cluster: Test
    user: mocks
  name: your-context
current-context: your-context
kind: Config
preferences: {}
users:
- name: mocks
  user:
    client-certificate-data: mocks
    client-key-data: mocks
`
