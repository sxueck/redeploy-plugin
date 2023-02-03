package main

import (
	"context"
	"fmt"
	"github.com/golang-module/carbon/v2"
	"github.com/labstack/echo/v4"
	rconfig "github.com/sxueck/k8sodep/config"
	"github.com/sxueck/k8sodep/model"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	"log"
	"net/http"
	"reflect"
)

var cfg = rconfig.Cfg
var now = carbon.Now().ToDateTimeString() // 2020-08-05 13:14:15

func NewInClusterClient() (*kubernetes.Clientset, error) {
	var (
		config *rest.Config
		err    error
	)

	apiServer := cfg.ApiServer
	if len(apiServer) > 0 {

		log.Printf("apiServer Address is : %s", apiServer)

		if cfg.KubeConfig == "" {
			return nil, fmt.Errorf("please configure KUBECONFIG parameters")
		}

		config, err = clientcmd.BuildConfigFromFlags("", cfg.KubeConfig)
		if err != nil {
			return nil, err
		}

	} else {
		//return nil, fmt.Errorf("please define API_SERVER parameter to specify k8s address")
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Printf("failed to obtain config from InClusterConfig: %v", err)
			return nil, err
		}
	}

	return kubernetes.NewForConfig(config)
}

func ReDeployWebhook(c echo.Context) error {
	var reCall = &model.ReCallDeployInfo{}
	err := c.Bind(&reCall)

	if err != nil {
		return c.String(http.StatusForbidden,
			fmt.Sprintf("bad format , %s", err))
	}

	if reCall.AccessToken != rconfig.Cfg.WebhookToken {
		return c.String(http.StatusForbidden, "TOKEN ERROR")
	}

	if len(reCall.Containers) == 0 {
		reCall.Containers = reCall.Resource
	}

	var containers []corev1.Container

	isDeployment := true

	kubeClient, err := NewInClusterClient()
	if err != nil {
		return c.String(http.StatusOK, err.Error())
	}

	deployment, err := kubeClient.
		AppsV1().
		Deployments(reCall.Namespace).
		Get(context.Background(), reCall.Resource, metav1.GetOptions{})

	var statefulSet *v1.StatefulSet
	if errors.IsNotFound(err) {
		log.Printf("%s statfulset", reCall.Resource)
		var getErr error
		statefulSet, getErr = kubeClient.
			AppsV1().
			StatefulSets(reCall.Namespace).
			Get(context.Background(), reCall.Resource, metav1.GetOptions{})

		if getErr != nil {
			log.Println(getErr)
			return c.String(http.StatusOK, getErr.Error())
		}

		containers = statefulSet.Spec.Template.Spec.Containers
		isDeployment = !isDeployment
	} else {
		if err != nil {
			return c.String(http.StatusOK, err.Error())
		}
		containers = deployment.Spec.Template.Spec.Containers
	}

	fmt.Printf("%+v\n", containers)

	found := false
	for i, v := range containers {
		if v.Name == reCall.Containers {
			found = true
			newImages := fmt.Sprintf("%s:%s", reCall.Images, reCall.Tag)
			log.Println("old images =>", v.Image)
			log.Println("new images =>", newImages)

			containers[i].Image = newImages
		}
	}

	if !found {
		return c.String(http.StatusOK,
			fmt.Sprintf("The application container not exist in the pod list"))
	}

	var deployERR error
	const annotationsKey = "redeploy.kubernetes.io/restartedAt"
	if isDeployment {

		// 通过更新 Annotations 时间戳，即使两个 Tag 相同，也能触发更新
		deployERR = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			deployment.
				Spec.
				Template.
				ObjectMeta.
				Annotations[annotationsKey] = now

			return postChangesMadeAfterSubmissionForCluster(kubeClient, reCall.Namespace, deployment)
		})

	} else {
		deployERR = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			statefulSet.
				Spec.
				Template.
				ObjectMeta.
				Annotations[annotationsKey] = now

			return postChangesMadeAfterSubmissionForCluster(kubeClient, reCall.Namespace, statefulSet)
		})
	}

	if deployERR != nil {
		return c.String(http.StatusOK, deployERR.Error())
	}

	return c.String(http.StatusOK, "successful")
}

func postChangesMadeAfterSubmissionForCluster[T *appsv1.Deployment | *appsv1.StatefulSet](
	kubeClient *kubernetes.Clientset, namespace string, res T) error {

	var err error
	rt := reflect.ValueOf(res).Interface()
	switch rt.(type) {
	case *appsv1.StatefulSet:
		// is statefulset
		_, err = kubeClient.
			AppsV1().
			StatefulSets(namespace).
			Update(context.Background(), rt.(*appsv1.StatefulSet), metav1.UpdateOptions{})
	case *appsv1.Deployment:
		// is deployment
		_, err = kubeClient.
			AppsV1().
			Deployments(namespace).
			Update(context.Background(), rt.(*appsv1.Deployment), metav1.UpdateOptions{})
	}
	return err
}
