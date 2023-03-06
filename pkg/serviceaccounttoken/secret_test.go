package serviceaccounttoken

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestEnsureSecretForServiceAccount(t *testing.T) {
	tests := []struct {
		name           string
		sa             *v1.ServiceAccount
		wantSA         *v1.ServiceAccount
		existingSecret *v1.Secret
		wantSecret     *v1.Secret
		wantErr        bool
	}{
		{
			name: "service account with no secret generates secret",
			sa: &v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			},
			wantSA: &v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Secrets: []v1.ObjectReference{{
					Name: "test-token-abcde",
				}},
			},
			wantSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-token-abcde",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"token": []byte("abcde"),
				},
			},
		},
		{
			name: "service account with existing secret returns it",
			sa: &v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Secrets: []v1.ObjectReference{{
					Name: "test-token-abcde",
				}},
			},
			wantSA: &v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Secrets: []v1.ObjectReference{{
					Name: "test-token-abcde",
				}},
			},
			existingSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-token-abcde",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"token": []byte("abcde"),
				},
			},
			wantSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-token-abcde",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"token": []byte("abcde"),
				},
			},
		},
		{
			name:    "returns error for nil service account",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var k8sClient *fake.Clientset
			objs := []runtime.Object{}
			if tt.sa != nil {
				objs = append(objs, tt.sa)
			}
			if tt.existingSecret != nil {
				objs = append(objs, tt.existingSecret)
			}
			k8sClient = fake.NewSimpleClientset(objs...)
			k8sClient.PrependReactor("create", "secrets",
				func(action k8stesting.Action) (bool, runtime.Object, error) {
					ret := action.(k8stesting.CreateAction).GetObject().(*v1.Secret)
					ret.ObjectMeta.Name = ret.GenerateName + "abcde"
					ret.Data = map[string][]byte{
						"token": []byte("abcde"),
					}
					return true, ret, nil
				},
			)
			got, gotErr := EnsureSecretForServiceAccount(context.Background(), nil, k8sClient, tt.sa)
			if tt.wantErr {
				assert.Error(t, gotErr)
				return
			}
			assert.NoError(t, gotErr)
			assert.Equal(t, tt.wantSecret.Name, got.Name)
			gotSA, _ := k8sClient.CoreV1().ServiceAccounts("default").Get(context.Background(), "test", metav1.GetOptions{})
			assert.Equal(t, tt.wantSA.Secrets, gotSA.Secrets)
		})
	}
}
