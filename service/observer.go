package service

import (
	"errors"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/Laica-Lunasys/oauth2-proxy-manager/models"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
)

type Observer struct {
	Clientset  *kubernetes.Clientset
	Controller *Controller
}

func NewObserver(clientset *kubernetes.Clientset, controller *Controller) (*Observer, error) {
	observer := &Observer{
		Clientset:  clientset,
		Controller: controller,
	}
	return observer, nil
}

func (ob *Observer) Run() {
	logrus.Info("[Observer] Observing Ingress...")

	// create resource watcher (ingress)
	watcher := cache.NewListWatchFromClient(ob.Clientset.ExtensionsV1beta1().RESTClient(), "ingresses", v1.NamespaceAll, fields.Everything())

	_, controller := cache.NewInformer(watcher, &v1beta1.Ingress{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				meta := obj.(*v1beta1.Ingress).ObjectMeta
				logrus.Infof("[Informer] Added Ingress %s", key)

				settings, err := parseAnnotations(meta)
				if err == nil {
					ob.Controller.Create(settings)
					//logrus.WithField("settings", settings).Info("Dummy: Update Deployment / ConfigMap / Service / Secret / Ingress")
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
					ob.Controller.Create(settings)
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
					logrus.WithField("settings", settings).Info("Dummy: Delete Deployment / ConfigMap / Service / Secret / Ingress")
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

	if _, ok := meta.Annotations["oauth2-proxy-manager.k8s.io/app-name"]; !ok {
		return nil, errors.New("app-name not found. skip.")
	}

	if _, ok := meta.Annotations["oauth2-proxy-manager.k8s.io/github-org"]; !ok {
		return nil, errors.New("github-org not found. skip.")
	}

	if _, ok := meta.Annotations["oauth2-proxy-manager.k8s.io/github-teams"]; !ok {
		return nil, errors.New("github-teams not found. skip.")
	}

	logrus.WithFields(logrus.Fields{
		"ingress.class": meta.Annotations["kubernetes.io/ingress.class"],
		"auth-url":      meta.Annotations["nginx.ingress.kubernetes.io/auth-url"],
		"auth-signin":   meta.Annotations["nginx.ingress.kubernetes.io/auth-signin"],
		"github-org":    meta.Annotations["oauth2-proxy-manager.k8s.io/github-org"],
		"github-teams":  meta.Annotations["oauth2-proxy-manager.k8s.io/github-teams"],
	}).Debug("[ParseAnnotations]")

	settings := &models.ServiceSettings{
		AppName:    meta.Annotations["oauth2-proxy-manager.k8s.io/app-name"],
		AuthURL:    meta.Annotations["nginx.ingress.kubernetes.io/auth-url"],
		AuthSignIn: meta.Annotations["nginx.ingress.kubernetes.io/auth-signin"],
		GitHub: models.GitHubProvider{
			Organization: meta.Annotations["oauth2-proxy-manager.k8s.io/github-org"],
			Teams:        strings.Split(meta.Annotations["oauth2-proxy-manager.k8s.io/github-teams"], ","),
		},
	}

	return settings, nil
}
