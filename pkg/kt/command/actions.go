package command

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/alibaba/kt-connect/pkg/kt/cluster"
	"github.com/alibaba/kt-connect/pkg/kt/connect"
	"github.com/alibaba/kt-connect/pkg/kt/options"
	"github.com/alibaba/kt-connect/pkg/kt/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Action cmd action
type Action struct {
	Options *options.DaemonOptions
}

// Connect connect vpn to kubernetes cluster
func (action *Action) Connect(options *options.DaemonOptions) (err error) {
	if util.IsDaemonRunning(options.RuntimeOptions.PidFile) {
		err = fmt.Errorf("Connect already running %s exit this", options.RuntimeOptions.PidFile)
		panic(err)
	}
	pid, err := util.WritePidFile(options.RuntimeOptions.PidFile)
	if err != nil {
		return
	}
	log.Info().Msgf("Connect Start At %d", pid)
	clientSet, err := cluster.GetKubernetesClient(options.KubeConfig)
	if err != nil {
		return
	}

	workload := fmt.Sprintf("kt-connect-daemon-%s", strings.ToLower(util.RandomString(5)))
	options.RuntimeOptions.Shadow = workload

	labels := map[string]string{
		"kt":           workload,
		"kt-component": "connect",
		"control-by":   "kt",
	}

	for k, v := range util.String2Map(options.Labels) {
		labels[k] = v
	}

	endPointIP, podName, err := cluster.CreateShadow(
		clientSet,
		workload,
		labels,
		options.Namespace,
		options.Image,
	)

	if err != nil {
		return
	}

	cidrs, err := util.GetCirds(clientSet, options.ConnectOptions.CIDR)
	if err != nil {
		return
	}

	connect.StartConnect(podName, endPointIP, cidrs, options)
	return
}

//Exchange exchange kubernetes workload
func (action *Action) Exchange(swap string, options *options.DaemonOptions) {
	checkConnectRunning(options.RuntimeOptions.PidFile)
	expose := options.ExchangeOptions.Expose

	if swap == "" || expose == "" {
		err := fmt.Errorf("-expose is required")
		panic(err.Error())
	}

	clientset, err := cluster.GetKubernetesClient(options.KubeConfig)
	if err != nil {
		panic(err.Error())
	}

	origin, err := clientset.AppsV1().Deployments(options.Namespace).Get(swap, metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	replicas := origin.Spec.Replicas

	// Prepare context inorder to remove after command exit
	options.RuntimeOptions.Origin = swap
	options.RuntimeOptions.Replicas = *replicas

	_, err = connect.Exchange(options, origin, clientset, util.String2Map(options.Labels))
	if err != nil {
		panic(err.Error())
	}
}

//Mesh exchange kubernetes workload
func (action *Action) Mesh(swap string, options *options.DaemonOptions) {
	checkConnectRunning(options.RuntimeOptions.PidFile)
	expose := options.MeshOptions.Expose

	if swap == "" || expose == "" {
		err := fmt.Errorf("-expose is required")
		panic(err.Error())
	}

	clientset, err := cluster.GetKubernetesClient(options.KubeConfig)
	if err != nil {
		panic(err.Error())
	}

	_, err = connect.Mesh(swap, options, clientset, util.String2Map(options.Labels))
	if err != nil {
		panic(err.Error())
	}
}

// checkConnectRunning check connect is running and print help msg
func checkConnectRunning(pidFile string) {
	daemonRunning := util.IsDaemonRunning(pidFile)
	if !daemonRunning {
		log.Info().Msgf("'KT Connect' not runing, you can only access local app from cluster")
	} else {
		log.Info().Msgf("'KT Connect' is runing, you can access local app from cluster and localhost")
	}
}
