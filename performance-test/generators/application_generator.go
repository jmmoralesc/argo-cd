package generator

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
)

type ApplicationGenerator struct {
	clientSet *appclientset.Clientset
}

func NewApplicationGenerator(clientSet *appclientset.Clientset) Generator {
	return &ApplicationGenerator{clientSet}
}

func (pg *ApplicationGenerator) Generate() error {
	applications := pg.clientSet.ArgoprojV1alpha1().Applications("argocd")
	_, err := applications.Create(context.TODO(), &v1alpha1.Application{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "application-",
			Namespace:    "argocd",
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Namespace: "argocd",
				Name:      "in-cluster",
			},
			Source: v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps",
				Path:           "helm-guestbook",
				TargetRevision: "master",
			},
		},
	}, v1.CreateOptions{})
	return err
}
