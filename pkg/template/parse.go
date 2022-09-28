package template

import (
	"bytes"
	"text/template"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Parse template
func Parse(templ string, in client.Object) (out []byte, err error) {
	tpl, err := template.New("horus").Parse(templ)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	if err := tpl.Execute(buf, in); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
