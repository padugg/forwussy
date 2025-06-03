package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	config, _ := rest.InClusterConfig()
	clientset, _ := kubernetes.NewForConfig(config)

	watcher, _ := clientset.CoreV1().Services("default").Watch(context.TODO(), metav1.ListOptions{})

	fmt.Println("🔍 Watching for LoadBalancer IP assignments...")

	for event := range watcher.ResultChan() {
		if event.Type != watch.Modified {
			continue
		}

		svc, ok := event.Object.(*v1.Service)
		if !ok || svc.Spec.Type != v1.ServiceTypeLoadBalancer {
			continue
		}

		ingress := svc.Status.LoadBalancer.Ingress
		if len(ingress) == 0 {
			continue
		}

		// You can store previous IPs if needed to detect new assignments
		fmt.Printf("🎯 Service %s/%s was assigned IP: %s\n", svc.Namespace, svc.Name, ingress[0].IP)

		port := svc.Spec.Ports[0]
		ip := ingress[0].IP
		name := svc.Name
		err := forwardPort(int(port.Port), ip, name)
		if err != nil {
			fmt.Println("Error: ", err.Error())
		}

	}
}

func forwardPort(port int, ip, name string) error {
	type forwardBody struct {
		Name            string   `json:"name"`
		Src             string   `json:"src"`
		Log             bool     `json:"log"`
		Proto           string   `json:"proto"`
		Fwd_port        string   `json:"fwd_port"`
		Fwd             string   `json:"fwd"`
		Dst_port        string   `json:"dst_port"`
		Enabled         bool     `json:"enabled"`
		Destination_ips []string `json:"destination_ips"`
		Destination_ip  string   `json:"destination_ip"`
		Pfwd_interface  string   `json:"pfwd_interface"`
	}

	router := os.Getenv("ROUTER_IP")
	apiEndpoint := os.Getenv("API_ENDPOINT")
	apiKey := os.Getenv("API_KEY")
	portStr := strconv.Itoa(port)

	body := &forwardBody{
		Name:            name,
		Src:             "any",
		Log:             false,
		Proto:           "tcp_udp",
		Fwd_port:        portStr,
		Dst_port:        portStr,
		Enabled:         true,
		Destination_ip:  ip,
		Fwd:             ip,
		Pfwd_interface:  "wan",
		Destination_ips: []string{},
	}

	bodyJson, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://"+router+apiEndpoint, bytes.NewBuffer(bodyJson))
	if err != nil {
		return err
	}
	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	_, err = client.Do(req)
	if err != nil {
		return err
	}

	return nil
}
