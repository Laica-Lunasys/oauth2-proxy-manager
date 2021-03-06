package main

import (
	"fmt"
	"os"
	"strings"

	appsv1beta2 "k8s.io/api/apps/v1beta2"
	apiv1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/Laica-Lunasys/oauth2-proxy-manager/logger"
	"github.com/Laica-Lunasys/oauth2-proxy-manager/models"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

type OAuth2Proxy struct {
	Clientset *kubernetes.Clientset
	Settings  *models.ServiceSettings
	Env       OAuth2ProxyEnv
	Ingress   IngressOption
}

type OAuth2ProxyEnv struct {
	Domain          string
	CookieDomain    string
	WhitelistDomain string
	Provider        string
	ClientID        string
	ClientSecret    string
}

type IngressOption struct {
	TLSSecretName string
	TLSHosts      string
	IngressClass  string
}

func makeOAuth2Proxy(clientset *kubernetes.Clientset, settings *models.ServiceSettings) OAuth2Proxy {
	return OAuth2Proxy{
		Clientset: clientset,
		Settings:  settings,
		Env: OAuth2ProxyEnv{
			Domain:          os.Getenv("OAUTH2_PROXY_DOMAIN"),
			CookieDomain:    os.Getenv("COOKIE_DOMAIN"),
			WhitelistDomain: os.Getenv("WHITELIST_DOMAIN"),
			Provider:        os.Getenv("PROVIDER"),
			ClientID:        os.Getenv("OAUTH2_PROXY_CLIENT_ID"),
			ClientSecret:    os.Getenv("OAUTH2_PROXY_CLIENT_SECRET"),
		},
		Ingress: IngressOption{
			IngressClass:  os.Getenv("INGRESS_CLASS"),
			TLSSecretName: os.Getenv("TLS_SECRET_NAME"),
			TLSHosts:      os.Getenv("TLS_HOSTS"),
		},
	}
}

func main() {
	logger.Init()

	// Example Model
	settings := &models.ServiceSettings{
		AppName: os.Getenv("APPNAME"),
		GitHub: models.GitHubProvider{
			Organization: os.Getenv("GITHUB_ORG"),
			Teams:        strings.Split(os.Getenv("GITHUB_TEAMS"), ","),
		},
	}

	logrus.Infof("[Showcase] Create oauth2_proxy(%s)...", settings.AppName)

	var config *rest.Config

	_, err := rest.InClusterConfig()
	if err != nil {
		// Not Cluster
		kubeconfig := os.Getenv("KUBECONFIG")
		if len(kubeconfig) == 0 {
			kubeconfig = os.Getenv("HOME") + "/.kube/config"
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			logrus.Panic(err)
		}
	} else {
		// In Cluster
		conf, err := rest.InClusterConfig()
		if err != nil {
			logrus.Fatalf("Failed get Kubernetes config: %v", err)
		}
		config = conf
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Fatal(err)
	}

	o := makeOAuth2Proxy(clientset, settings)
	o.applyService()
	o.applySecret()
	o.applyConfigMap()
	o.applyDeployment()
	o.applyIngress()

}

func (o *OAuth2Proxy) applyService() {
	servicesClient := o.Clientset.CoreV1().Services("oauth2-proxy")
	service := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("oauth2-proxy-github-%s-%s", o.Settings.GitHub.Organization, o.Settings.AppName),
			Namespace: "oauth2-proxy",
			Annotations: map[string]string{
				"kubernetes.io/ingress.class": "nginx",
			},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeNodePort,
			Ports: []apiv1.ServicePort{
				apiv1.ServicePort{
					Name:       "http",
					Port:       80,
					Protocol:   apiv1.ProtocolTCP,
					TargetPort: intstr.FromString("http"),
				},
			},
			Selector: map[string]string{
				"app": fmt.Sprintf("oauth2-proxy-github-%s-%s", o.Settings.GitHub.Organization, o.Settings.AppName),
			},
		},
	}
	logrus.Printf("[oauth2_proxy] Check Service...")
	result, err := servicesClient.Get(fmt.Sprintf("oauth2-proxy-github-%s-%s", o.Settings.GitHub.Organization, o.Settings.AppName), metav1.GetOptions{})
	if len(result.GetName()) == 0 {
		// NotFound
		logrus.Printf("[oauth2_proxy] Creating Service...")
		result, err = servicesClient.Create(service)
		if err != nil {
			logrus.Panic(err)
		}
		logrus.Printf("[oauth2_proxy] Created Service! %q", result.GetObjectMeta().GetName())
	} else {
		logrus.Printf("[oauth2_proxy] Update Service...")

		// Inject ClusterIP
		logrus.Debugf("[oauth2_proxy] Detected ClusterIP: %s", result.Spec.ClusterIP)
		service.Spec.ClusterIP = result.Spec.ClusterIP

		// Inject ResourceVersion
		logrus.Debugf("[oauth2_proxy] Detected ResourceVersion: %s", result.GetResourceVersion())
		service.SetResourceVersion(result.GetResourceVersion())
		result, err = servicesClient.Update(service)
		if err != nil {
			logrus.Panic(err)
		}
		logrus.Printf("[oauth2_proxy] Updated Service! %q", result.GetObjectMeta().GetName())
	}

}

func (o *OAuth2Proxy) applyIngress() {
	ingressClient := o.Clientset.ExtensionsV1beta1().Ingresses("oauth2-proxy")
	ingress := &extensionsv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oauth2-proxy",
			Namespace: "oauth2-proxy",
		},
		Spec: extensionsv1beta1.IngressSpec{
			Rules: []extensionsv1beta1.IngressRule{
				extensionsv1beta1.IngressRule{
					Host: o.Env.Domain,
					IngressRuleValue: extensionsv1beta1.IngressRuleValue{
						HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
							Paths: []extensionsv1beta1.HTTPIngressPath{
								extensionsv1beta1.HTTPIngressPath{
									Path: fmt.Sprintf("/github/%s", o.Settings.AppName),
									Backend: extensionsv1beta1.IngressBackend{
										ServiceName: fmt.Sprintf("oauth2-proxy-%s-%s-%s", o.Env.Provider, o.Settings.GitHub.Organization, o.Settings.AppName),
										ServicePort: intstr.FromInt(80),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if len(o.Ingress.TLSHosts) != 0 && len(o.Ingress.TLSSecretName) != 0 {
		ingress.Spec.TLS = []extensionsv1beta1.IngressTLS{
			extensionsv1beta1.IngressTLS{
				Hosts:      strings.Split(o.Ingress.TLSHosts, ","),
				SecretName: o.Ingress.TLSSecretName,
			},
		}
	}

	result, err := ingressClient.Get("oauth2-proxy", metav1.GetOptions{})
	if len(result.GetName()) == 0 {
		// NotFound
		logrus.Printf("[oauth2_proxy] Creating Ingress...")

		result, err = ingressClient.Create(ingress)
		if err != nil {
			logrus.Panic(err)
		}
		logrus.Printf("[oauth2_proxy] Created Ingress! %q", result.GetObjectMeta().GetName())
	} else {
		logrus.Printf("[oauth2_proxy] Update Ingress...")

		// Append New Entry
		for _, existPath := range result.Spec.Rules[0].IngressRuleValue.HTTP.Paths {
			if existPath.Path != ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Path {
				ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths = append(ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths, existPath)
			}
		}

		result, err = ingressClient.Update(ingress)
		if err != nil {
			logrus.Panic(err)
		}
		logrus.Printf("[oauth2_proxy] Updated Ingress! %q", result.GetObjectMeta().GetName())
	}
}

func (o *OAuth2Proxy) applySecret() {
	secretClient := o.Clientset.CoreV1().Secrets("oauth2-proxy")
	secret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("oauth2-proxy-github-%s-%s", o.Settings.GitHub.Organization, o.Settings.AppName),
			Namespace: "oauth2-proxy",
		},
		Type: apiv1.SecretTypeOpaque,
		StringData: map[string]string{
			fmt.Sprintf("%s-%s-%s-cookie-secret", o.Env.Provider, o.Settings.GitHub.Organization, o.Settings.AppName): "PLEASERANDOM",
			"client-secret": o.Env.ClientSecret,
			"client-id":     o.Env.ClientID,
		},
	}
	logrus.Printf("[oauth2_proxy] Check Secret...")
	result, err := secretClient.Get(fmt.Sprintf("oauth2-proxy-github-%s-%s", o.Settings.GitHub.Organization, o.Settings.AppName), metav1.GetOptions{})
	if len(result.GetName()) == 0 {
		// NotFound
		logrus.Printf("[oauth2_proxy] Creating Secret...")
		result, err = secretClient.Create(secret)
		if err != nil {
			logrus.Panic(err)
		}
		logrus.Printf("[oauth2_proxy] Created Secret! %q", result.GetObjectMeta().GetName())
	} else {
		logrus.Printf("[oauth2_proxy] Update Secret...")
		result, err = secretClient.Update(secret)
		if err != nil {
			logrus.Panic(err)
		}
		logrus.Printf("[oauth2_proxy] Updated Secret! %q", result.GetObjectMeta().GetName())
	}
}

func (o *OAuth2Proxy) applyConfigMap() {
	configMapClient := o.Clientset.CoreV1().ConfigMaps("oauth2-proxy")
	configMap := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("oauth2-proxy-github-%s-%s", o.Settings.GitHub.Organization, o.Settings.AppName),
			Namespace: "oauth2-proxy",
		},
		Data: map[string]string{
			"oauth2_proxy.cfg": "email_domains = [ \"*\" ]\nupstreams = [ \"file:///dev/null\" ]",
		},
	}
	logrus.Printf("[oauth2_proxy] Check ConfigMap...")
	result, err := configMapClient.Get(fmt.Sprintf("oauth2-proxy-github-%s-%s", o.Settings.GitHub.Organization, o.Settings.AppName), metav1.GetOptions{})
	if len(result.GetName()) == 0 {
		// NotFound
		logrus.Printf("[oauth2_proxy] Creating ConfigMap...")
		result, err = configMapClient.Create(configMap)
		if err != nil {
			logrus.Panic(err)
		}
		logrus.Printf("[oauth2_proxy] Created ConfigMap! %q", result.GetObjectMeta().GetName())
	} else {
		logrus.Printf("[oauth2_proxy] Update ConfigMap...")
		result, err = configMapClient.Update(configMap)
		if err != nil {
			logrus.Panic(err)
		}
		logrus.Printf("[oauth2_proxy] Updated ConfigMap! %q", result.GetObjectMeta().GetName())
	}
}

func (o *OAuth2Proxy) applyDeployment() {
	deploymentsClient := o.Clientset.AppsV1beta2().Deployments("oauth2-proxy")
	deployment := &appsv1beta2.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("oauth2-proxy-github-%s-%s", o.Settings.GitHub.Organization, o.Settings.AppName),
			Namespace: "oauth2-proxy",
		},
		Spec: appsv1beta2.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": fmt.Sprintf("oauth2-proxy-github-%s-%s", o.Settings.GitHub.Organization, o.Settings.AppName),
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": fmt.Sprintf("oauth2-proxy-github-%s-%s", o.Settings.GitHub.Organization, o.Settings.AppName),
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						apiv1.Container{
							Name:  "oauth2-proxy",
							Image: "quay.io/pusher/oauth2_proxy:v3.2.0",
							Args: []string{
								"--http-address=0.0.0.0:4180",
								fmt.Sprintf("--cookie-domain=%s", o.Env.CookieDomain),
								fmt.Sprintf("--cookie-name=_github_%s_%s_oauth2_proxy", o.Settings.GitHub.Organization, o.Settings.AppName),
								"--email-domain=*",
								fmt.Sprintf("--github-org=%s", o.Settings.GitHub.Organization),
								fmt.Sprintf("--github-team=%s", strings.Join(o.Settings.GitHub.Teams, ",")),
								fmt.Sprintf("--provider=github"),
								fmt.Sprintf("--proxy-prefix=/github/%s", o.Settings.AppName),
								fmt.Sprintf("--redirect-url=https://%s/github/%s/callback", o.Env.Domain, o.Settings.AppName),
								fmt.Sprintf("--upstream=file:///dev/null"),
								fmt.Sprintf("--whitelist-domain=%s", o.Env.WhitelistDomain),
								fmt.Sprintf("--config=/etc/oauth2_proxy/oauth2_proxy.cfg"),
							},
							Env: []apiv1.EnvVar{
								apiv1.EnvVar{
									Name: "OAUTH2_PROXY_CLIENT_ID",
									ValueFrom: &apiv1.EnvVarSource{
										SecretKeyRef: &apiv1.SecretKeySelector{
											LocalObjectReference: apiv1.LocalObjectReference{
												Name: fmt.Sprintf("oauth2-proxy-%s-%s-%s", o.Env.Provider, o.Settings.GitHub.Organization, o.Settings.AppName),
											},
											Key: "client-id",
										},
									},
								},
								apiv1.EnvVar{
									Name: "OAUTH2_PROXY_CLIENT_SECRET",
									ValueFrom: &apiv1.EnvVarSource{
										SecretKeyRef: &apiv1.SecretKeySelector{
											LocalObjectReference: apiv1.LocalObjectReference{
												Name: fmt.Sprintf("oauth2-proxy-%s-%s-%s", o.Env.Provider, o.Settings.GitHub.Organization, o.Settings.AppName),
											},
											Key: "client-secret",
										},
									},
								},
								apiv1.EnvVar{
									Name: "OAUTH2_PROXY_COOKIE_SECRET",
									ValueFrom: &apiv1.EnvVarSource{
										SecretKeyRef: &apiv1.SecretKeySelector{
											LocalObjectReference: apiv1.LocalObjectReference{
												Name: fmt.Sprintf("oauth2-proxy-%s-%s-%s", o.Env.Provider, o.Settings.GitHub.Organization, o.Settings.AppName),
											},
											Key: fmt.Sprintf("%s-%s-%s-cookie-secret", o.Env.Provider, o.Settings.GitHub.Organization, o.Settings.AppName),
										},
									},
								},
							},
							Ports: []apiv1.ContainerPort{
								{
									Name:          "http",
									Protocol:      apiv1.ProtocolTCP,
									ContainerPort: 4180,
								},
							},
							LivenessProbe: &apiv1.Probe{
								InitialDelaySeconds: 0,
								TimeoutSeconds:      1,
								Handler: apiv1.Handler{
									HTTPGet: &apiv1.HTTPGetAction{
										Path: "/ping",
										Port: intstr.FromString("http"),
									},
								},
							},
							ReadinessProbe: &apiv1.Probe{
								InitialDelaySeconds: 0,
								TimeoutSeconds:      1,
								SuccessThreshold:    1,
								PeriodSeconds:       10,
								Handler: apiv1.Handler{
									HTTPGet: &apiv1.HTTPGetAction{
										Path: "/ping",
										Port: intstr.FromString("http"),
									},
								},
							},
							VolumeMounts: []apiv1.VolumeMount{
								apiv1.VolumeMount{
									Name:      "configmain",
									MountPath: "/etc/oauth2_proxy",
								},
							},
						},
					},
					Volumes: []apiv1.Volume{
						apiv1.Volume{
							Name: "configmain",
							VolumeSource: apiv1.VolumeSource{
								ConfigMap: &apiv1.ConfigMapVolumeSource{
									DefaultMode: int32Ptr(420),
									LocalObjectReference: apiv1.LocalObjectReference{
										Name: fmt.Sprintf("oauth2-proxy-%s-%s-%s", o.Env.Provider, o.Settings.GitHub.Organization, o.Settings.AppName),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Create deployment...
	logrus.Printf("[oauth2_proxy] Check Deployment...")
	result, err := deploymentsClient.Get(fmt.Sprintf("oauth2-proxy-github-%s-%s", o.Settings.GitHub.Organization, o.Settings.AppName), metav1.GetOptions{})
	if len(result.GetName()) == 0 {
		// NotFound
		logrus.Printf("[oauth2_proxy] Creating Deployment...")
		result, err = deploymentsClient.Create(deployment)
		if err != nil {
			logrus.Panic(err)
		}
		logrus.Printf("[oauth2_proxy] Created Deployment! %q", result.GetObjectMeta().GetName())
	} else {
		logrus.Printf("[oauth2_proxy] Update Deployment...")
		result, err = deploymentsClient.Update(deployment)
		if err != nil {
			logrus.Panic(err)
		}
		logrus.Printf("[oauth2_proxy] Updated Deployment! %q", result.GetObjectMeta().GetName())
	}
}

func int32Ptr(i int32) *int32 { return &i }
