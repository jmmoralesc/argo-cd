package generators

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	crtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
)

type genProcessorMock struct {
	mock.Mock
}

func (g *genProcessorMock) Transform(requestedGenerator argoprojiov1alpha1.ApplicationSetGenerator, allGenerators map[string]Generator, baseTemplate argoprojiov1alpha1.ApplicationSetTemplate, appSet *argoprojiov1alpha1.ApplicationSet, genParams []map[string]string) ([]TransformResult, error) {
	args := g.Called(requestedGenerator)

	return args.Get(0).([]TransformResult), args.Error(1)
}

func (g *genProcessorMock) GetRelevantGenerators(requestedGenerator argoprojiov1alpha1.ApplicationSetGenerator, generators map[string]Generator) []Generator {
	args := g.Called(requestedGenerator)

	return args.Get(0).([]Generator)
}

func (g *genProcessorMock) interpolateGenerator(requestedGenerator *argoprojiov1alpha1.ApplicationSetGenerator, params []map[string]string) error {
	args := g.Called(requestedGenerator)

	return args.Error(1)
}

func getMockClusterGenerator() Generator {
	clusters := []crtclient.Object{
		&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "staging-01",
				Namespace: "namespace",
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
					"environment":                    "staging",
					"org":                            "foo",
				},
				Annotations: map[string]string{
					"foo.argoproj.io": "staging",
				},
			},
			Data: map[string][]byte{
				"config": []byte("{}"),
				"name":   []byte("staging-01"),
				"server": []byte("https://staging-01.example.com"),
			},
			Type: corev1.SecretType("Opaque"),
		},
		&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "production-01",
				Namespace: "namespace",
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
					"environment":                    "production",
					"org":                            "bar",
				},
				Annotations: map[string]string{
					"foo.argoproj.io": "production",
				},
			},
			Data: map[string][]byte{
				"config": []byte("{}"),
				"name":   []byte("production_01/west"),
				"server": []byte("https://production-01.example.com"),
			},
			Type: corev1.SecretType("Opaque"),
		},
	}
	runtimeClusters := []runtime.Object{}
	appClientset := kubefake.NewSimpleClientset(runtimeClusters...)

	fakeClient := fake.NewClientBuilder().WithObjects(clusters...).Build()
	return NewClusterGenerator(fakeClient, context.Background(), appClientset, "namespace")
}

func getMockGitGenerator() Generator {
	cases := []struct {
		name          string
		directories   []argoprojiov1alpha1.GitDirectoryGeneratorItem
		repoApps      []string
		repoError     error
		expected      []map[string]string
		expectedError error
	}{
		{
			name:        "happy flow - created apps",
			directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			repoApps: []string{
				"app1",
				"app2",
				"app_3",
				"p1/app4",
			},
			repoError: nil,
			expected: []map[string]string{
				{"path": "app1", "path.basename": "app1", "path.basenameNormalized": "app1"},
				{"path": "app2", "path.basename": "app2", "path.basenameNormalized": "app2"},
				{"path": "app_3", "path.basename": "app_3", "path.basenameNormalized": "app-3"},
			},
			expectedError: nil,
		},
		{
			name:        "It filters application according to the paths",
			directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "p1/*"}, {Path: "p1/*/*"}},
			repoApps: []string{
				"app1",
				"p1/app2",
				"p1/p2/app3",
				"p1/p2/p3/app4",
			},
			repoError: nil,
			expected: []map[string]string{
				{"path": "p1/app2", "path.basename": "app2", "path[0]": "p1", "path.basenameNormalized": "app2"},
				{"path": "p1/p2/app3", "path.basename": "app3", "path[0]": "p1", "path[1]": "p2", "path.basenameNormalized": "app3"},
			},
			expectedError: nil,
		},
		{
			name:        "It filters application according to the paths with Exclude",
			directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "p1/*", Exclude: true}, {Path: "*"}, {Path: "*/*"}},
			repoApps: []string{
				"app1",
				"app2",
				"p1/app2",
				"p1/app3",
				"p2/app3",
			},
			repoError: nil,
			expected: []map[string]string{
				{"path": "app1", "path.basename": "app1", "path.basenameNormalized": "app1"},
				{"path": "app2", "path.basename": "app2", "path.basenameNormalized": "app2"},
				{"path": "p2/app3", "path.basename": "app3", "path[0]": "p2", "path.basenameNormalized": "app3"},
			},
			expectedError: nil,
		},
		{
			name:        "Expecting same exclude behavior with different order",
			directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}, {Path: "*/*"}, {Path: "p1/*", Exclude: true}},
			repoApps: []string{
				"app1",
				"app2",
				"p1/app2",
				"p1/app3",
				"p2/app3",
			},
			repoError: nil,
			expected: []map[string]string{
				{"path": "app1", "path.basename": "app1", "path.basenameNormalized": "app1"},
				{"path": "app2", "path.basename": "app2", "path.basenameNormalized": "app2"},
				{"path": "p2/app3", "path.basename": "app3", "path[0]": "p2", "path.basenameNormalized": "app3"},
			},
			expectedError: nil,
		},
		{
			name:          "handles empty response from repo server",
			directories:   []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     nil,
			expected:      []map[string]string{},
			expectedError: nil,
		},
		{
			name:          "handles error from repo server",
			directories:   []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
			repoApps:      []string{},
			repoError:     fmt.Errorf("error"),
			expected:      []map[string]string{},
			expectedError: fmt.Errorf("error"),
		},
	}
	argoCDServiceMock := argoCDServiceMock{mock: &mock.Mock{}}

	argoCDServiceMock.mock.On("GetDirectories", mock.Anything, mock.Anything, mock.Anything).Return(cases[0].repoApps, cases[0].repoError)

	var gitGenerator = NewGitGenerator(argoCDServiceMock)
	return gitGenerator
}

func TestGetRelevantGenerators(t *testing.T) {

	testGenerators := map[string]Generator{
		"Clusters": getMockClusterGenerator(),
		"Git":      getMockGitGenerator(),
	}

	testGenerators["Matrix"] = NewMatrixGenerator(testGenerators)
	testGenerators["Merge"] = NewMergeGenerator(testGenerators)
	testGenerators["List"] = NewListGenerator()

	requestedGenerator := &argoprojiov1alpha1.ApplicationSetGenerator{
		List: &argoprojiov1alpha1.ListGenerator{
			Elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "cluster","url": "url","values":{"foo":"bar"}}`)}},
		}}

	relevantGenerators := GetRelevantGenerators(requestedGenerator, testGenerators)
	assert.Len(t, relevantGenerators, 1)
	assert.IsType(t, &ListGenerator{}, relevantGenerators[0])

	requestedGenerator = &argoprojiov1alpha1.ApplicationSetGenerator{
		Clusters: &argoprojiov1alpha1.ClusterGenerator{
			Selector: metav1.LabelSelector{},
			Template: argoprojiov1alpha1.ApplicationSetTemplate{},
			Values:   nil,
		},
	}

	relevantGenerators = GetRelevantGenerators(requestedGenerator, testGenerators)
	assert.Len(t, relevantGenerators, 1)
	assert.IsType(t, &ClusterGenerator{}, relevantGenerators[0])

	requestedGenerator = &argoprojiov1alpha1.ApplicationSetGenerator{
		Git: &argoprojiov1alpha1.GitGenerator{
			RepoURL:             "",
			Directories:         nil,
			Files:               nil,
			Revision:            "",
			RequeueAfterSeconds: nil,
			Template:            argoprojiov1alpha1.ApplicationSetTemplate{},
		},
	}

	relevantGenerators = GetRelevantGenerators(requestedGenerator, testGenerators)
	assert.Len(t, relevantGenerators, 1)
	assert.IsType(t, &GitGenerator{}, relevantGenerators[0])
}

func TestInterpolateGenerator(t *testing.T) {
	requestedGenerator := &argoprojiov1alpha1.ApplicationSetGenerator{
		Clusters: &argoprojiov1alpha1.ClusterGenerator{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
					"path-basename":                  "{{path.basename}}",
					"path-zero":                      "{{path[0]}}",
					"path-full":                      "{{path}}",
				}},
		},
	}
	//mockGitGenerator := getMockGitGenerator()
	gitGeneratorParams := []map[string]string{
		{"path": "p1/p2/app3", "path.basename": "app3", "path[0]": "p1", "path[1]": "p2", "path.basenameNormalized": "app3"},
	}
	err := interpolateGenerator(requestedGenerator, gitGeneratorParams)
	if err != nil {
		log.WithError(err).WithField("requestedGenerator", requestedGenerator).Error("error interpolating Generator")
		return
	}
	assert.Equal(t, "app3", requestedGenerator.Clusters.Selector.MatchLabels["path-basename"])
	assert.Equal(t, "p1", requestedGenerator.Clusters.Selector.MatchLabels["path-zero"])
	assert.Equal(t, "p1/p2/app3", requestedGenerator.Clusters.Selector.MatchLabels["path-full"])

	fileNamePath := argoprojiov1alpha1.GitFileGeneratorItem{
		Path: "{{name}}",
	}
	fileServerPath := argoprojiov1alpha1.GitFileGeneratorItem{
		Path: "{{server}}",
	}

	requestedGenerator = &argoprojiov1alpha1.ApplicationSetGenerator{
		Git: &argoprojiov1alpha1.GitGenerator{
			Files:    append([]argoprojiov1alpha1.GitFileGeneratorItem{}, fileNamePath, fileServerPath),
			Template: argoprojiov1alpha1.ApplicationSetTemplate{},
		},
	}
	clusterGeneratorParams := []map[string]string{
		{"name": "production_01/west", "server": "https://production-01.example.com"},
	}
	err = interpolateGenerator(requestedGenerator, clusterGeneratorParams)
	if err != nil {
		log.WithError(err).WithField("requestedGenerator", requestedGenerator).Error("error interpolating Generator")
		return
	}
	assert.Equal(t, "production_01/west", requestedGenerator.Git.Files[0].Path)
	assert.Equal(t, "https://production-01.example.com", requestedGenerator.Git.Files[1].Path)
}
