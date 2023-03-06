package dashboard

import (
	"context"
	"strings"

	"github.com/rancher/rancher/pkg/features"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/rancher/rancher/pkg/settings"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultURL = "https://git.rancher.io/"

var (
	prefix = "rancher-"
)

func addRepo(wrangler *wrangler.Context, repoName, repoURL, branchName string) error {
	if repoURL == "" || repoURL == defaultURL {
		repoURL = defaultURL + strings.TrimPrefix(repoName, prefix)
	}
	repo, err := wrangler.Catalog.ClusterRepo().Get(repoName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = wrangler.Catalog.ClusterRepo().Create(&v1.ClusterRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name: repoName,
			},
			Spec: v1.RepoSpec{
				GitRepo:   repoURL,
				GitBranch: branchName,
			},
		})
	} else if err == nil && (repo.Spec.GitBranch != branchName || repo.Spec.GitRepo != repoURL) {
		repo.Spec.GitRepo = repoURL
		repo.Spec.GitBranch = branchName
		_, err = wrangler.Catalog.ClusterRepo().Update(repo)
	}

	return err
}

func addRepos(ctx context.Context, wrangler *wrangler.Context) error {
	if err := addRepo(wrangler, "rancher-charts", settings.ChartDefaultURL.Get(), settings.ChartDefaultBranch.Get()); err != nil {
		return err
	}
	if err := addRepo(wrangler, "rancher-partner-charts", defaultURL, settings.PartnerChartDefaultBranch.Get()); err != nil {
		return err
	}

	if features.RKE2.Enabled() {
		if err := addRepo(wrangler, "rancher-rke2-charts", defaultURL, settings.RKE2ChartDefaultBranch.Get()); err != nil {
			return err
		}
	}

	return nil
}
