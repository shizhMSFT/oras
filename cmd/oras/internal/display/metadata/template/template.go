/*
Copyright The ORAS Authors.
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

package template

import (
	"io"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"oras.land/oras/cmd/oras/internal/display/utils"
)

func parseAndWrite(out io.Writer, object any, templateStr string) error {
	// parse template
	t, err := template.New("format output").Funcs(sprig.FuncMap()).Parse(templateStr)
	if err != nil {
		return err
	}
	// convert object to map[string]any
	converted, err := utils.ToMap(object)
	if err != nil {
		return err
	}
	return t.Execute(out, converted)
}
