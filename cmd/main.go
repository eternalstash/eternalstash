package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	clientset := getKubernetesClient()

	factory := informers.NewSharedInformerFactory(clientset, time.Second*30)
	podInformer := factory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			printPodInfo("ADDED", pod)
		},
		DeleteFunc: func(obj interface{}) {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					fmt.Printf("Couldn't get object from tombstone %+v\n", obj)
					return
				}
				pod, ok = tombstone.Obj.(*corev1.Pod)
				if !ok {
					fmt.Printf("Tombstone contained object that is not a Pod %+v\n", obj)
					return
				}
			}
			printPodInfo("DELETED", pod)
		},
	})

	// Start the informer to begin watching for Pod events
	stopCh := make(chan struct{})
	defer close(stopCh)

	go podInformer.Run(stopCh)

	// Wait for the informer to sync
	if !cache.WaitForCacheSync(stopCh, podInformer.HasSynced) {
		fmt.Println("Timed out waiting for caches to sync")
		return
	}

	// Wait forever to keep the program running
	select {}
}

func getKubernetesClient() *kubernetes.Clientset {
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
	return clientset
}

func printPodInfo(eventType string, pod *corev1.Pod) {
	if pod == nil {
		fmt.Println("Received nil pod object.")
		return
	}

	fmt.Printf("%s - Pod Name: %s, Namespace: %s\n", eventType, pod.Name, pod.Namespace)

	for _, containerStatus := range pod.Status.ContainerStatuses {
		fmt.Printf("  Container Name: %s, Image: %s, ImageID: %s \n", containerStatus.Name, containerStatus.Image, containerStatus.ImageID)
	}

	if eventType == "ADDED" && pod.Status.StartTime != nil {
		fmt.Printf("  StartAt: %s \n", pod.Status.StartTime.Format(time.RFC3339))
	} else if eventType == "DELETED" {
		fmt.Printf("  StartAt: %s, Deleted At: %s \n", pod.Status.StartTime.Format(time.RFC3339), time.Now().Format(time.RFC3339))
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
