package main

import (
	"errors"
	"flag"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/Laica-Lunasys/oauth2-proxy-manager/logger"
	"github.com/Laica-Lunasys/oauth2-proxy-manager/models"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	resource := "ingresses"

	logger.Init()

	logrus.Info("[Showcase] Observing Ingress...")
	var kubeconfig string
	var master string

	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
	flag.Parse()

	// creates the connection
	config, err := clientcmd.BuildConfigFromFlags(master, kubeconfig)
	if err != nil {
		logrus.Fatal(err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Fatal(err)
	}

	// create resource watcher (ingress)
	watcher := cache.NewListWatchFromClient(clientset.ExtensionsV1beta1().RESTClient(), resource, v1.NamespaceAll, fields.Everything())

	_, controller := cache.NewInformer(watcher, &v1beta1.Ingress{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				meta := obj.(*v1beta1.Ingress).ObjectMeta
				logrus.Infof("[Informer] Added Ingress %s", key)

				settings, err := parseAnnotations(meta)
				if err == nil {
					logrus.WithField("settings", settings).Info("Dummy: Update Deployment / ConfigMap / Service / Secret / Ingress")
				}
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)

			if err == nil {
				meta := new.(*v1beta1.Ingress).ObjectMeta
				logrus.Infof("[Informer] Update Ingress %s", key)

				settings, err := parseAnnotations(meta)
				if err == nil {
					logrus.WithField("settings", settings).Info("Dummy: Update Deployment / ConfigMap / Service / Secret / Ingress")
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				meta := obj.(*v1beta1.Ingress).ObjectMeta
				logrus.Infof("[Informer] Delete Ingress: %s", key)

				settings, err := parseAnnotations(meta)
				if err == nil {
					logrus.WithField("settings", settings).Info("Dummy: Update Deployment / ConfigMap / Service / Secret / Ingress")
				}
			}
		},
	})

	// Now let's start the controller
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)

	// Wait forever
	select {}
}

func parseAnnotations(meta metav1.ObjectMeta) (*models.ServiceSettings, error) {
	// Check Annotations ---
	if _, ok := meta.Annotations["kubernetes.io/ingress.class"]; !ok {
		return nil, errors.New("ingress.class not found. skip.")
	} else if meta.Annotations["kubernetes.io/ingress.class"] != "nginx" {
		// or ingress.class is "nginx" ?
		return nil, errors.New("ingress.class is not nginx. skip.")
	}

	if _, ok := meta.Annotations["nginx.ingress.kubernetes.io/auth-url"]; !ok {
		return nil, errors.New("auth-url not found. skip.")
	}

	if _, ok := meta.Annotations["nginx.ingress.kubernetes.io/auth-signin"]; !ok {
		return nil, errors.New("auth-signin not found. skip.")
	}

	if _, ok := meta.Annotations["oauth2-proxy-manager.lunasys.dev/github-org"]; !ok {
		return nil, errors.New("git/ub-org not found. skip.")
	}

	if _, ok := meta.Annotations["oauth2-proxy-manager.lunasys.dev/github-teams"]; !ok {
		return nil, errors.New("github-teams not found. skip.")
	}

	logrus.WithFields(logrus.Fields{
		"ingress.class": meta.Annotations["kubernetes.io/ingress.class"],
		"auth-url":      meta.Annotations["nginx.ingress.kubernetes.io/auth-url"],
		"auth-signin":   meta.Annotations["nginx.ingress.kubernetes.io/auth-signin"],
		"github-org":    meta.Annotations["oauth2-proxy-manager.lunasys.dev/github-org"],
		"github-teams":  meta.Annotations["oauth2-proxy-manager.lunasys.dev/github-teams"],
	}).Debug("[ParseAnnotations]")

	settings := &models.ServiceSettings{
		IngressClass: meta.Annotations["kubernetes.io/ingress.class"],
		AuthURL:      meta.Annotations["nginx.ingress.kubernetes.io/auth-url"],
		AuthSignIn:   meta.Annotations["nginx.ingress.kubernetes.io/auth-signin"],
		GitHub: models.GitHubProvider{
			Organization: meta.Annotations["oauth2-proxy-manager.lunasys.dev/github-org"],
			Teams:        strings.Split(meta.Annotations["oauth2-proxy-manager.lunasys.dev/github-teams"], ","),
		},
	}

	return settings, nil
}
