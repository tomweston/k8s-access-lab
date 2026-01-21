package main

import (
	"os"
	"strconv"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	networkingv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/networking/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")

		namespace := "app-nginx"

		host := cfg.Get("host")
		if host == "" {
			host = "nginx.example.com"
		}

		sslRedirect := true
		if sslRedirectRaw := cfg.Get("sslRedirect"); sslRedirectRaw != "" {
			parsed, err := strconv.ParseBool(sslRedirectRaw)
			if err != nil {
				return err
			}
			sslRedirect = parsed
		}

		image := cfg.Get("image")
		if image == "" {
			image = "nginx:1.25-alpine"
		}

		replicas := cfg.GetInt("replicas")
		if replicas == 0 {
			replicas = 2
		}

		kubeconfigPath := cfg.Get("kubeconfig")
		var provider *kubernetes.Provider
		var err error
		if kubeconfigPath != "" {
			// Read the file content
			kubeconfigContent, err := os.ReadFile(kubeconfigPath)
			if err != nil {
				return err
			}
			provider, err = kubernetes.NewProvider(ctx, "k8s", &kubernetes.ProviderArgs{
				Kubeconfig: pulumi.String(string(kubeconfigContent)),
			})
			if err != nil {
				return err
			}
		}

		opts := []pulumi.ResourceOption{}
		if provider != nil {
			opts = append(opts, pulumi.Provider(provider))
		}

		labels := pulumi.StringMap{
			"app.kubernetes.io/name": pulumi.String("nginx"),
		}

		htmlBytes, err := os.ReadFile("index.html")
		if err != nil {
			return err
		}
		html := string(htmlBytes)

		_, err = corev1.NewConfigMap(ctx, "nginx-html", &corev1.ConfigMapArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String("nginx-html"),
				Namespace: pulumi.String(namespace),
				Labels:    labels,
			},
			Data: pulumi.StringMap{
				"index.html": pulumi.String(html),
			},
		}, opts...)
		if err != nil {
			return err
		}

		_, err = appsv1.NewDeployment(ctx, "nginx", &appsv1.DeploymentArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String("nginx"),
				Namespace: pulumi.String(namespace),
				Labels:    labels,
			},
			Spec: &appsv1.DeploymentSpecArgs{
				Replicas: pulumi.Int(replicas),
				Selector: &metav1.LabelSelectorArgs{
					MatchLabels: labels,
				},
				Template: &corev1.PodTemplateSpecArgs{
					Metadata: &metav1.ObjectMetaArgs{
						Labels: labels,
					},
					Spec: &corev1.PodSpecArgs{
						Containers: corev1.ContainerArray{
							&corev1.ContainerArgs{
								Name:            pulumi.String("nginx"),
								Image:           pulumi.String(image),
								ImagePullPolicy: pulumi.String("IfNotPresent"),
								Ports: corev1.ContainerPortArray{
									&corev1.ContainerPortArgs{
										Name:          pulumi.String("http"),
										ContainerPort: pulumi.Int(80),
										Protocol:      pulumi.String("TCP"),
									},
								},
								VolumeMounts: corev1.VolumeMountArray{
									&corev1.VolumeMountArgs{
										Name:      pulumi.String("html-volume"),
										MountPath: pulumi.String("/usr/share/nginx/html"),
									},
								},
							},
						},
						Volumes: corev1.VolumeArray{
							&corev1.VolumeArgs{
								Name: pulumi.String("html-volume"),
								ConfigMap: &corev1.ConfigMapVolumeSourceArgs{
									Name: pulumi.String("nginx-html"),
								},
							},
						},
					},
				},
			},
		}, opts...)
		if err != nil {
			return err
		}

		_, err = corev1.NewService(ctx, "nginx", &corev1.ServiceArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String("nginx"),
				Namespace: pulumi.String(namespace),
				Labels:    labels,
			},
			Spec: &corev1.ServiceSpecArgs{
				Type: pulumi.String("ClusterIP"),
				Ports: corev1.ServicePortArray{
					&corev1.ServicePortArgs{
						Port:       pulumi.Int(80),
						TargetPort: pulumi.String("http"),
						Protocol:   pulumi.String("TCP"),
						Name:       pulumi.String("http"),
					},
				},
				Selector: labels,
			},
		}, opts...)
		if err != nil {
			return err
		}

		annotations := pulumi.StringMap{
			"cert-manager.io/cluster-issuer": pulumi.String("self-signed-issuer"),
		}
		if !sslRedirect {
			annotations["nginx.ingress.kubernetes.io/ssl-redirect"] = pulumi.String("false")
			annotations["nginx.ingress.kubernetes.io/force-ssl-redirect"] = pulumi.String("false")
		}

		_, err = networkingv1.NewIngress(ctx, "nginx", &networkingv1.IngressArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:        pulumi.String("nginx"),
				Namespace:   pulumi.String(namespace),
				Labels:      labels,
				Annotations: annotations,
			},
			Spec: &networkingv1.IngressSpecArgs{
				IngressClassName: pulumi.String("nginx"),
				Tls: networkingv1.IngressTLSArray{
					&networkingv1.IngressTLSArgs{
						Hosts: pulumi.StringArray{
							pulumi.String(host),
						},
						SecretName: pulumi.String("nginx-tls"),
					},
				},
				Rules: networkingv1.IngressRuleArray{
					&networkingv1.IngressRuleArgs{
						Host: pulumi.String(host),
						Http: &networkingv1.HTTPIngressRuleValueArgs{
							Paths: networkingv1.HTTPIngressPathArray{
								&networkingv1.HTTPIngressPathArgs{
									Path:     pulumi.String("/"),
									PathType: pulumi.String("Prefix"),
									Backend: &networkingv1.IngressBackendArgs{
										Service: &networkingv1.IngressServiceBackendArgs{
											Name: pulumi.String("nginx"),
											Port: &networkingv1.ServiceBackendPortArgs{
												Number: pulumi.Int(80),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}, opts...)
		if err != nil {
			return err
		}

		ctx.Export("namespace", pulumi.String(namespace))
		ctx.Export("host", pulumi.String(host))
		ctx.Export("replicas", pulumi.Int(replicas))
		ctx.Export("image", pulumi.String(image))
		ctx.Export("provider", pulumi.String(func() string {
			if kubeconfigPath != "" {
				return "pulumi-provider"
			}
			return "default-kubeconfig"
		}()))

		return nil
	})
}
