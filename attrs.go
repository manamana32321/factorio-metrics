package main

import "go.opentelemetry.io/otel/attribute"

func nameAttribute(name string) attribute.KeyValue {
	return attribute.String("name", name)
}
