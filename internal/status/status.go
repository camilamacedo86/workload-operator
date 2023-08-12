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

// Package status defines the conditional status that will be used by this project
package status

// ConditionAvailable indicates that the associated custom resource is available and operating as intended.
// A resource is considered Available when the system's components are correctly configured
// and ready to perform their tasks.
const ConditionAvailable = "Available"

// ConditionDegraded indicates that the custom resource is in a degraded state.
// This usually means that an error has occurred and the resource is not fully functional,
// but it is not completely inoperative.
const ConditionDegraded = "Degraded"

// ConditionProgressing indicates that the custom resource is currently being applied or updated.
// This condition is set when changes to the configuration have been accepted but not yet completed.
const ConditionProgressing = "Progressing"
