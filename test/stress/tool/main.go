package main

import (
	"context"
	"fmt"
	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"math/rand"
	"sigs.k8s.io/yaml"
	"time"
)

var (
	wtClient v1alpha1.WorkflowTemplateInterface
	wfClient v1alpha1.WorkflowInterface
	ctx      context.Context
)

func init() {
	var (
		config *rest.Config
		err    error
	)
	config, err = clientcmd.BuildConfigFromFlags("", "config/kubeconfig.yaml")
	if err != nil {
		panic(err)
	}
	namespace := "argo-workflow"
	wtClient = versioned.NewForConfigOrDie(config).ArgoprojV1alpha1().WorkflowTemplates(namespace)
	wfClient = versioned.NewForConfigOrDie(config).ArgoprojV1alpha1().Workflows(namespace)
	ctx = context.Background()
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func createTemplate(yamlPath string) {

	wt := &wfv1.WorkflowTemplate{}
	yamlFile, err := ioutil.ReadFile(yamlPath)
	checkError(err)
	err = yaml.Unmarshal(yamlFile, wt)
	checkError(err)

	workflow, err := wtClient.Get(ctx, wt.GetObjectMeta().GetName(), metav1.GetOptions{})
	if err != nil {
		_, err = wtClient.Create(ctx, wt, metav1.CreateOptions{})
		checkError(err)
	} else {
		// update workflow if exist
		// MUST set meta ResourceVersion when update
		wt.ObjectMeta.ResourceVersion = workflow.ResourceVersion
		_, err = wtClient.Update(ctx, wt, metav1.UpdateOptions{})
		checkError(err)
	}
}

func createWorkflow(yamlPath string, n int) {
	wf := &wfv1.Workflow{}

	yamlFile, err := ioutil.ReadFile(yamlPath)
	checkError(err)
	err = yaml.Unmarshal(yamlFile, wf)
	checkError(err)
	wf.GenerateName = ""

	i := 0
	for i = 0; i < n; i++ {
		wf.Name = fmt.Sprintf("stress-%v", i)
		for j := 0; j < 5; j++ {
			_, errWhenCreate := wfClient.Create(ctx, wf, metav1.CreateOptions{})
			if errWhenCreate == nil {
				break
			}
			err = errWhenCreate
		}
		if err != nil {
			print(i, " ")
			break
		}
		if i%10 == 0 {
			print(i, " ")
		}
	}
}
func countDuration(n int, sleepSeconds int) {
	time.Sleep(time.Duration(rand.Int31n(int32(sleepSeconds))) * time.Second)
	println("")
	var waitDuration int64
	var runDuration int64
	allExists := false
	i := 0
	var wfNames = make(map[string]bool)

	for i < n {
		name := fmt.Sprintf("stress-%v", i)
		wfNames[name] = false
		i += 1
	}
	i = 0
	for !allExists {
		allExists = true
		for name, exists := range wfNames {
			allExists = allExists && exists
			if exists {
				if sleepSeconds/100 > 0 {
					time.Sleep(time.Duration(rand.Int31n(int32(sleepSeconds/100))) * time.Second)
				}
				continue
			}
			workflow, err := wfClient.Get(ctx, name, metav1.GetOptions{})
			checkError(err)
			if workflow.Status.Phase.Completed() {
				wfNames[name] = true
				runDuration += workflow.Status.GetDuration().Milliseconds()
				waitDuration += workflow.Status.StartedAt.UnixMilli() - workflow.ObjectMeta.CreationTimestamp.UnixMilli()
				if i%10 == 0 {
					print(i, ":", name, " ")
				}
				i += 1
			}
		}
	}

	println("")
	i = 0
	for name, _ := range wfNames {
		_ = wfClient.Delete(ctx, name, metav1.DeleteOptions{})
		if i%10 == 0 {
			print(i, ":", name, " ")
		}
		i += 1
	}
	println("")
	println(fmt.Sprintf("avg wait duration: %v", waitDuration/int64(n)))
	println(fmt.Sprintf("avg run duration: %v", runDuration/int64(n)))
}

func main() {
	countDuration(500, 5)
	for _, n := range []int{800, 1000, 1500, 2000, 3000, 4000, 5000, 6000, 7000, 8000} {
		createTemplate("test/stress/massive-workflow-template.yaml")
		createWorkflow("test/stress/massive-workflow-sleep-2hour.yaml", n)
		countDuration(n, 7200)
	}
}
