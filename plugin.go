package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type (
	Repo struct {
		Owner string
		Name  string
	}

	Build struct {
		Tag     string
		Event   string
		Number  int
		Commit  string
		Ref     string
		Branch  string
		Author  string
		Status  string
		Link    string
		Started int64
		Created int64
	}

	Job struct {
		Started int64
	}

	Config struct {
		Ca        string
		Server    string
		Token     string
		Namespace string
		Template  string
	}

	Plugin struct {
		Repo   Repo
		Build  Build
		Config Config
		Job    Job
	}
)

func (p Plugin) Exec() error {

	if p.Config.Server == "" {
		log.Fatal("KUBE_SERVER is not defined")
	}
	if p.Config.Token == "" {
		log.Fatal("KUBE_TOKEN is not defined")
	}
	if p.Config.Ca == "" {
		log.Fatal("KUBE_CA is not defined")
	}
	if p.Config.Namespace == "" {
		p.Config.Namespace = "default"
	}
	if p.Config.Template == "" {
		log.Fatal("KUBE_TEMPLATE, or template must be defined")
	}

	// connect to Kubernetes
	clientset, err := p.createKubeClient()
	if err != nil {
		log.Fatal(err.Error())
	}

	// parse the template file and do substitutions
	txt, err := openAndSub(p.Config.Template, p)
	if err != nil {
		return err
	}
	// convert txt back to []byte and convert to json
	json, err := yaml.ToJSON([]byte(txt))
	if err != nil {
		return err
	}

	var dep appsv1.Deployment

	e := runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), json, &dep)
	if e != nil {
		log.Fatal("Error decoding yaml file to json", e)
	}
	// check and see if there is a deployment already.  If there is, update it.
	oldDep, err := findDeployment(dep.ObjectMeta.Name, dep.ObjectMeta.Namespace, clientset)
	if err != nil {
		return err
	}
	if oldDep.ObjectMeta.Name == dep.ObjectMeta.Name {
		// update the existing deployment, ignore the deployment that it comes back with
		_, err = clientset.AppsV1().Deployments(dep.ObjectMeta.Namespace).Update(&dep)
		if err != nil {
			return err
		}
		log.Printf("Updated deployment %s", oldDep.ObjectMeta.Name)
		return nil
	}
	// create the new deployment since this never existed.
	_, err = clientset.AppsV1().Deployments(dep.ObjectMeta.Namespace).Create(&dep)
	if err != nil {
		return err
	}
	log.Printf("Updated deployment %s", dep.ObjectMeta.Name)
	return nil
}

func findDeployment(depName string, namespace string, c *kubernetes.Clientset) (appsv1.Deployment, error) {
	if namespace == "" {
		namespace = "default"
	}
	var d appsv1.Deployment
	deployments, err := listDeployments(c, namespace)
	if err != nil {
		return d, err
	}
	for _, thisDep := range deployments {
		if thisDep.ObjectMeta.Name == depName {
			return thisDep, err
		}
	}
	return d, err
}

// List the deployments
func listDeployments(clientset *kubernetes.Clientset, namespace string) ([]appsv1.Deployment, error) {
	deployments, err := clientset.AppsV1().Deployments(namespace).List(metav1.ListOptions{})
	if err != nil {
		log.Fatal(err.Error())
	}
	return deployments.Items, err
}

// open up the template and then sub variables in. Handlebar stuff.
func openAndSub(templateFile string, p Plugin) (string, error) {
	t, err := ioutil.ReadFile(templateFile)
	if err != nil {
		return "", err
	}
	//potty humor!  Render trim toilet paper!  Ha ha, so funny.
	return RenderTrim(string(t), p)
}

// create the connection to kubernetes based on parameters passed in.
// the kubernetes/client-go project is really hard to understand.
func (p Plugin) createKubeClient() (*kubernetes.Clientset, error) {

	ca, err := base64.StdEncoding.DecodeString(p.Config.Ca)
	if err != nil {
		return nil, err
	}
	config := clientcmdapi.NewConfig()
	config.Clusters["drone"] = &clientcmdapi.Cluster{
		Server: p.Config.Server,
		CertificateAuthorityData: ca,
	}
	token, err := base64.StdEncoding.DecodeString(p.Config.Token)
	if err != nil {
		return nil, err
	}
	config.AuthInfos["drone"] = &clientcmdapi.AuthInfo{
		Token: string(token),
	}

	config.Contexts["drone"] = &clientcmdapi.Context{
		Cluster:  "drone",
		AuthInfo: "drone",
	}
	//config.Clusters["drone"].CertificateAuthorityData = ca
	config.CurrentContext = "drone"

	clientBuilder := clientcmd.NewNonInteractiveClientConfig(*config, "drone", &clientcmd.ConfigOverrides{}, nil)
	actualCfg, err := clientBuilder.ClientConfig()
	if err != nil {
		log.Fatal(err)
	}
	return kubernetes.NewForConfig(actualCfg)

}

// Just an example from the client specification.  Code not really used.
func watchPodCounts(clientset *kubernetes.Clientset) {
	for {
		pods, err := clientset.Core().Pods("").List(v1.ListOptions{})
		if err != nil {
			log.Fatal(err.Error())
		}
		fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))
		time.Sleep(10 * time.Second)
	}
}
