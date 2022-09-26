package util

import (
	"fmt"

	"github.com/forbearing/k8s/util/labels"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

/*
doc:
	https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/

app.kubernetes.io/name			The name of the application
app.kubernetes.io/instance		A unique name identifying the instance of an application
app.kubernetes.io/version		The current version of the application (e.g., a semantic version, revision hash, etc.)
app.kubernetes.io/component		The component within the architecture
app.kubernetes.io/part-of		The name of a higher level application this one is part of
app.kubernetes.io/managed-by	The tool being used to manage the operation of an application

apiVersion: apps/v1
kind: StatefulSet
metadata:
  labels:
    app.kubernetes.io/name: mysql
    app.kubernetes.io/instance: mysql-abcxzy
    app.kubernetes.io/version: "5.7.21"
    app.kubernetes.io/component: database
    app.kubernetes.io/part-of: wordpress
    app.kubernetes.io/managed-by: helm
*/

var (
	LabelPairNoIstioSiecar = "sidecar.istio.io/inject=false"
)

var LabelMap = map[string]string{
	"app":                          "horus",
	"app.kubernetes.io/name":       "horus",
	"app.kubernetes.io/instance":   "",
	"app.kubernetes.io/component":  "",
	"app.kubernetes.io/part-of":    "horus",
	"app.kubernetes.io/managed-by": "horus-operator",
}

func WithRecommendedLabels(object runtime.Object) {
	if object == nil {
		return
	}
	for key, value := range LabelMap {
		pair := fmt.Sprintf("%s=%s", key, value)
		if err := labels.Set(object, pair); err != nil {
			logrus.Errorf("set recommended labels failed: %s", err)
			break
		}
	}
}
