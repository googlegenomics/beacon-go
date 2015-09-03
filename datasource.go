/*
 * Copyright (C) 2015 Google Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 */

package beacon

type DataSource struct {
	URL            string
	APIKeyFileName string
	VariantSetIds  []string
}

// The list of DataSources to search.
var DataSources = []DataSource{
	{
		URL:            "https://www.googleapis.com/genomics/v1beta2/variants/search?",
		APIKeyFileName: "google_api_key.txt",
		VariantSetIds:  []string{"10473108253681171589"},
	},
	{
		URL:            "https://www.googleapis.com/genomics/v1beta2/variants/search?",
		APIKeyFileName: "google_api_key.txt",
		VariantSetIds:  []string{"3049512673186936334"},
	},
}
