package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	kubeconfig := flag.String("kubeconfig", getKubeconfigPath(), "path to kubeconfig file")
	flag.Parse()

	config, err := getClientConfig(*kubeconfig)
	if err != nil {
		fmt.Printf("Error getting client config: %v\n", err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error creating Kubernetes client: %v\n", err)
		os.Exit(1)
	}

	watcher, _ := clientset.CoreV1().Pods(corev1.NamespaceAll).
		Watch(context.Background(), metav1.ListOptions{})
	for event := range watcher.ResultChan() {
		pod := event.Object.(*corev1.Pod)
		fmt.Printf("%v pod with name %s in namespace %s\n", event.Type, pod.Name, pod.Namespace)
	}

}

func getKubeconfigPath() string {
	home := homedir.HomeDir()
	return home + "/.kube/config"
}

func getClientConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}
