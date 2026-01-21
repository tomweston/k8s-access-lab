package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	apiextensions "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apiextensions"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	helmv3 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/rbac/v1"
	k8syaml "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/yaml"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	certsv1 "k8s.io/api/certificates/v1"
	metav1k8s "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func kubeClientFromKubeconfig(path string) (*k8sclient.Clientset, *clientcmdapi.Config, error) {
	cfg, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return nil, nil, err
	}
	restCfg, err := clientcmd.BuildConfigFromFlags("", path)
	if err != nil {
		return nil, nil, err
	}
	clientset, err := k8sclient.NewForConfig(restCfg)
	if err != nil {
		return nil, nil, err
	}
	return clientset, cfg, nil
}

func generateKeyAndCSR(cn, org string) ([]byte, []byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	csrTemplate := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: []string{org},
		},
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &csrTemplate, key)
	if err != nil {
		return nil, nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	csrPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrDER,
	})
	return keyPEM, csrPEM, nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")

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

		// Install Flannel CNI
		_, err = k8syaml.NewConfigFile(ctx, "flannel", &k8syaml.ConfigFileArgs{
			File: "https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml",
		}, opts...)
		if err != nil {
			return err
		}

		// Install Ingress-Nginx via Helm
		ingressRelease, err := helmv3.NewRelease(ctx, "ingress-nginx", &helmv3.ReleaseArgs{
			Chart:   pulumi.String("ingress-nginx"),
			Version: pulumi.String("4.10.0"), // Pin version for stability
			RepositoryOpts: &helmv3.RepositoryOptsArgs{
				Repo: pulumi.String("https://kubernetes.github.io/ingress-nginx"),
			},
			Namespace:       pulumi.String("ingress-nginx"),
			CreateNamespace: pulumi.Bool(true),
			Values: pulumi.Map{
				"controller": pulumi.Map{
					"service": pulumi.Map{
						"type": pulumi.String("NodePort"),
					},
					"hostNetwork":              pulumi.Bool(true),
					"watchIngressWithoutClass": pulumi.Bool(true),
				},
			},
		}, opts...)
		if err != nil {
			return err
		}

		// Install Cert-Manager via Helm
		certManagerRelease, err := helmv3.NewRelease(ctx, "cert-manager", &helmv3.ReleaseArgs{
			Chart:   pulumi.String("cert-manager"),
			Version: pulumi.String("v1.15.3"),
			RepositoryOpts: &helmv3.RepositoryOptsArgs{
				Repo: pulumi.String("https://charts.jetstack.io"),
			},
			Namespace:       pulumi.String("cert-manager"),
			CreateNamespace: pulumi.Bool(true),
			Values: pulumi.Map{
				"installCRDs": pulumi.Bool(true),
			},
		}, opts...)
		if err != nil {
			return err
		}

		// Create the namespace for the app
		ns, err := corev1.NewNamespace(ctx, "app-namespace", &corev1.NamespaceArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("app-nginx"),
			},
		}, opts...)
		if err != nil {
			return err
		}

		// Create the role for the nginx-deployer user
		_, err = rbacv1.NewRole(ctx, "nginx-deployer-role", &rbacv1.RoleArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String("nginx-deployer-role"),
				Namespace: ns.Metadata.Name(),
			},
			Rules: rbacv1.PolicyRuleArray{
				&rbacv1.PolicyRuleArgs{
					ApiGroups: pulumi.StringArray{
						pulumi.String(""),
						pulumi.String("apps"),
						pulumi.String("networking.k8s.io"),
					},
					Resources: pulumi.StringArray{
						pulumi.String("pods"),
						pulumi.String("pods/log"),
						pulumi.String("services"),
						pulumi.String("configmaps"),
						pulumi.String("secrets"),
						pulumi.String("deployments"),
						pulumi.String("replicasets"),
						pulumi.String("ingresses"),
					},
					Verbs: pulumi.StringArray{
						pulumi.String("get"),
						pulumi.String("list"),
						pulumi.String("watch"),
						pulumi.String("create"),
						pulumi.String("update"),
						pulumi.String("patch"),
						pulumi.String("delete"),
					},
				},
				&rbacv1.PolicyRuleArgs{
					ApiGroups: pulumi.StringArray{pulumi.String("cert-manager.io")},
					Resources: pulumi.StringArray{pulumi.String("certificates"), pulumi.String("certificaterequests")},
					Verbs: pulumi.StringArray{
						pulumi.String("get"),
						pulumi.String("list"),
						pulumi.String("watch"),
						pulumi.String("create"),
						pulumi.String("update"),
						pulumi.String("patch"),
						pulumi.String("delete"),
					},
				},
			},
		}, opts...)
		if err != nil {
			return err
		}

		_, err = rbacv1.NewRoleBinding(ctx, "nginx-deployer-binding", &rbacv1.RoleBindingArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String("nginx-deployer-binding"),
				Namespace: ns.Metadata.Name(),
			},
			RoleRef: &rbacv1.RoleRefArgs{
				ApiGroup: pulumi.String("rbac.authorization.k8s.io"),
				Kind:     pulumi.String("Role"),
				Name:     pulumi.String("nginx-deployer-role"),
			},
			Subjects: rbacv1.SubjectArray{
				&rbacv1.SubjectArgs{
					Kind:     pulumi.String("User"),
					Name:     pulumi.String("nginx-deployer"),
					ApiGroup: pulumi.String("rbac.authorization.k8s.io"),
				},
			},
		}, opts...)
		if err != nil {
			return err
		}

		issuerOpts := append(opts, pulumi.DependsOn([]pulumi.Resource{certManagerRelease, ingressRelease}))
		_, err = apiextensions.NewCustomResource(ctx, "self-signed-issuer", &apiextensions.CustomResourceArgs{
			ApiVersion: pulumi.String("cert-manager.io/v1"),
			Kind:       pulumi.String("ClusterIssuer"),
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("self-signed-issuer"),
			},
			OtherFields: kubernetes.UntypedArgs{
				"spec": map[string]interface{}{
					"selfSigned": map[string]interface{}{},
				},
			},
		}, issuerOpts...)
		if err != nil {
			return err
		}

		clientset, adminCfg, err := kubeClientFromKubeconfig(kubeconfigPath)
		if err != nil {
			return err
		}

		csrName := "nginx-deployer-csr"
		keyPEM, csrDER, err := generateKeyAndCSR("nginx-deployer", "nginx-deployers")
		if err != nil {
			return err
		}

		csrReq := &certsv1.CertificateSigningRequest{
			ObjectMeta: metav1k8s.ObjectMeta{
				Name: csrName,
			},
			Spec: certsv1.CertificateSigningRequestSpec{
				Request:    csrDER,
				SignerName: certsv1.KubeAPIServerClientSignerName,
				Usages:     []certsv1.KeyUsage{certsv1.UsageClientAuth},
			},
		}

		ctxBackground := context.Background()
		if _, err := clientset.CertificatesV1().CertificateSigningRequests().Get(ctxBackground, csrName, metav1k8s.GetOptions{}); err == nil {
			_ = clientset.CertificatesV1().CertificateSigningRequests().Delete(ctxBackground, csrName, metav1k8s.DeleteOptions{})
		}

		if _, err := clientset.CertificatesV1().CertificateSigningRequests().Create(ctxBackground, csrReq, metav1k8s.CreateOptions{}); err != nil {
			return err
		}

		approval := csrReq.DeepCopy()
		approval.Status.Conditions = append(approval.Status.Conditions, certsv1.CertificateSigningRequestCondition{
			Type:           certsv1.CertificateApproved,
			Status:         "True",
			Reason:         "PulumiAdminApproval",
			Message:        "Approved by admin stack",
			LastUpdateTime: metav1k8s.Now(),
		})
		if _, err := clientset.CertificatesV1().CertificateSigningRequests().UpdateApproval(ctxBackground, csrName, approval, metav1k8s.UpdateOptions{}); err != nil {
			return err
		}

		var certBytes []byte
		for i := 0; i < 30; i++ {
			csr, err := clientset.CertificatesV1().CertificateSigningRequests().Get(ctxBackground, csrName, metav1k8s.GetOptions{})
			if err != nil {
				return err
			}
			if len(csr.Status.Certificate) > 0 {
				certBytes = csr.Status.Certificate
				break
			}
			time.Sleep(2 * time.Second)
		}
		if len(certBytes) == 0 {
			return fmt.Errorf("timed out waiting for CSR certificate")
		}

		clusterName := adminCfg.Contexts[adminCfg.CurrentContext].Cluster
		cluster := adminCfg.Clusters[clusterName]
		server := cluster.Server
		kubeConfig := clientcmdapi.NewConfig()
		kubeConfig.Clusters[clusterName] = &clientcmdapi.Cluster{
			Server:                   server,
			CertificateAuthorityData: cluster.CertificateAuthorityData,
		}
		kubeConfig.AuthInfos["nginx-deployer"] = &clientcmdapi.AuthInfo{
			ClientCertificateData: certBytes,
			ClientKeyData:         keyPEM,
		}
		kubeConfig.Contexts["nginx-deployer-context"] = &clientcmdapi.Context{
			Cluster:   clusterName,
			AuthInfo:  "nginx-deployer",
			Namespace: "app-nginx",
		}
		kubeConfig.CurrentContext = "nginx-deployer-context"
		kubeconfigBytes, err := clientcmd.Write(*kubeConfig)
		if err != nil {
			return err
		}
		nginxKubeconfig := string(kubeconfigBytes)

		ctx.Export("namespace", ns.Metadata.Name())
		ctx.Export("nginxDeployerKubeconfig", pulumi.ToSecret(pulumi.String(nginxKubeconfig)))
		return nil
	})
}
