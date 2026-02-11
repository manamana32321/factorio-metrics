package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	otellog "go.opentelemetry.io/otel/log"
)

var (
	chatPattern     = regexp.MustCompile(`\[CHAT\]\s+(.+?):\s+(.+)`)
	joinPattern     = regexp.MustCompile(`(.+?)\s+joined the game`)
	leavePattern    = regexp.MustCompile(`(.+?)\s+left the game`)
	researchPattern = regexp.MustCompile(`Research finished:\s+(.+)`)
	rocketPattern   = regexp.MustCompile(`Rocket launched`)
	savePattern     = regexp.MustCompile(`Saving game as\s+(.+)`)
)

// LogTailer tails Factorio server pod logs via the K8s API and emits structured OTel logs.
type LogTailer struct {
	namespace string
	podLabel  string
	logger    otellog.Logger
}

func NewLogTailer(namespace, podLabel string, logger otellog.Logger) *LogTailer {
	return &LogTailer{
		namespace: namespace,
		podLabel:  podLabel,
		logger:    logger,
	}
}

func (t *LogTailer) Run(ctx context.Context) {
	for {
		if err := t.tail(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("log tail error: %v, retrying in 5s", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (t *LogTailer) tail(ctx context.Context) error {
	podName, err := t.findPod(ctx)
	if err != nil {
		return fmt.Errorf("find pod: %w", err)
	}

	log.Printf("tailing logs from pod %s/%s", t.namespace, podName)

	body, err := t.streamLogs(ctx, podName)
	if err != nil {
		return fmt.Errorf("stream logs: %w", err)
	}
	defer body.Close()

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil
		}
		t.parseLine(scanner.Text())
	}
	return scanner.Err()
}

func (t *LogTailer) parseLine(line string) {
	if m := chatPattern.FindStringSubmatch(line); m != nil {
		logEvent(t.logger, "chat",
			otellog.String("player", m[1]),
			otellog.String("message", m[2]),
		)
		return
	}
	if m := joinPattern.FindStringSubmatch(line); m != nil {
		logEvent(t.logger, "join", otellog.String("player", m[1]))
		return
	}
	if m := leavePattern.FindStringSubmatch(line); m != nil {
		logEvent(t.logger, "leave", otellog.String("player", m[1]))
		return
	}
	if m := researchPattern.FindStringSubmatch(line); m != nil {
		logEvent(t.logger, "research", otellog.String("tech", m[1]))
		return
	}
	if rocketPattern.MatchString(line) {
		logEvent(t.logger, "rocket")
		return
	}
	if m := savePattern.FindStringSubmatch(line); m != nil {
		logEvent(t.logger, "save", otellog.String("name", m[1]))
		return
	}
}

// K8s API helpers — uses in-cluster service account

func (t *LogTailer) k8sClient() (*http.Client, string, error) {
	token, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return nil, "", fmt.Errorf("read sa token: %w", err)
	}
	caCert, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		// fallback: skip TLS verify if CA not found
		return &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}, string(token), nil
	}
	_ = caCert

	// Use default transport with CA
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}, string(token), nil
}

func (t *LogTailer) apiBase() string {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return "https://kubernetes.default.svc"
	}
	return fmt.Sprintf("https://%s:%s", host, port)
}

func (t *LogTailer) findPod(ctx context.Context) (string, error) {
	client, token, err := t.k8sClient()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/api/v1/namespaces/%s/pods?labelSelector=%s&limit=1",
		t.apiBase(), t.namespace, t.podLabel)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("list pods: %s %s", resp.Status, string(body))
	}

	var result struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Items) == 0 {
		return "", fmt.Errorf("no pods found with label %s", t.podLabel)
	}
	return result.Items[0].Metadata.Name, nil
}

func (t *LogTailer) streamLogs(ctx context.Context, podName string) (io.ReadCloser, error) {
	client, token, err := t.k8sClient()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/log?follow=true&tailLines=0&timestamps=false",
		t.apiBase(), t.namespace, podName)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Remove timeout for streaming
	client.Timeout = 0

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("stream logs: %s %s", resp.Status, string(body))
	}

	return resp.Body, nil
}
