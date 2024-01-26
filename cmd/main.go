package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var coll *mongo.Collection

func main() {
	clientset := getKubernetesClient()

	factory := informers.NewSharedInformerFactory(clientset, time.Second*30)
	podInformer := factory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			printPodInfo("ADDED", pod)
			savePodInfo("ADDED", pod)
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
			savePodInfo("DELETED", pod)
		},
	})

	// Connect to mongoDB
	// if err := godotenv.Load(); err != nil {
	// 	log.Println("No .env file found")
	// }
	// uri := os.Getenv("MONGODB_URI")
	// if uri == "" {
	// 	log.Fatal("You must set your 'MONGODB_URI' environment variable. See\n\t https://www.mongodb.com/docs/drivers/go/current/usage-examples/#environment-variable")
	// }
	uri := "mongodb://localhost:27017"
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	coll = client.Database("eternalstash").Collection("images")

	// Start the informer to begin watching for Pod events
	stopCh := make(chan struct{})
	defer close(stopCh)

	go podInformer.Run(stopCh)

	// Wait for the informer to sync
	if !cache.WaitForCacheSync(stopCh, podInformer.HasSynced) {
		fmt.Println("Timed out waiting for caches to sync")
		return
	}

	router := gin.Default()
	router.GET("/images", getImages)

	router.Run("localhost:8080")

	// Wait forever to keep the program running
	// select {}
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

type Image struct {
	Pod       string `json:"pod"`
	Container string `json:"container"`
	Image     string `json:"image"`
	ImageID   string `json:"imageId"`
	Namespace string `json:"namespace"`
	StartedAt string `json:"startedAt"`
	DeletedAt string `json:"deletedAt"`
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

func savePodInfo(eventType string, pod *corev1.Pod) {
	if pod == nil {
		fmt.Println("Received nil pod object.")
		return
	}
	for _, containerStatus := range pod.Status.ContainerStatuses {
		deletedAt := ""
		if eventType == "DELETED" {
			deletedAt = time.Now().Format(time.RFC3339)
		}
		image := Image{
			Pod:       pod.Name,
			Namespace: pod.Namespace,
			Container: containerStatus.Name,
			Image:     containerStatus.Image,
			ImageID:   containerStatus.ImageID,
			StartedAt: pod.Status.StartTime.Format(time.RFC3339),
			DeletedAt: deletedAt,
		}
		result, err := coll.InsertOne(context.TODO(), image)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Saved result %v\n", &result.InsertedID)
	}
}

func getImages(c *gin.Context) {
	cursor, err := coll.Find(context.TODO(), bson.D{})
	if err != nil {
		panic(err)
	}
	// end find

	var results []Image
	if err = cursor.All(context.TODO(), &results); err != nil {
		panic(err)
	}

	c.IndentedJSON(http.StatusOK, results)
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
