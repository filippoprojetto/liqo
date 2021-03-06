/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"github.com/coreos/go-iptables/iptables"
	clusterConfig "github.com/liqotech/liqo/apis/config/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/internal/liqonet"
	"github.com/liqotech/liqo/pkg/liqonet"
	"github.com/vishvananda/netlink"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"net"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"strconv"
	"strings"
	"time"
	// +kubebuilder:scaffold:imports
)

var (
	scheme        = runtime.NewScheme()
	defaultConfig = liqonet.VxlanNetConfig{
		Network:    "192.168.200.0/24",
		DeviceName: "liqonet",
		Port:       "4789", //IANA assigned
		Vni:        "200",
	}
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = netv1alpha1.AddToScheme(scheme)

}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var runAsRouteOperator bool
	var runAs string

	flag.StringVar(&metricsAddr, "metrics-addr", ":0", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&runAsRouteOperator, "run-as-route-operator", false,
		"Runs the controller as Route-Operator, the default value is false and will run as Tunnel-Operator")
	flag.StringVar(&runAs, "run-as", "tunnel-operator", "The accepted values are: tunnel-operator, route-operator, tunnelEndpointCreator-operator. The default value is \"tunnel-operator\"")
	flag.Parse()
	waitCleanUp := make(chan struct{})
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		Port:               9443,
	})
	if err != nil {
		klog.Errorf("unable to get manager: %s", err)
		os.Exit(1)
	}
	// creates the in-cluster config or uses the .kube/config file
	config := ctrl.GetConfigOrDie()
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	// +kubebuilder:scaffold:builder
	switch runAs {
	case "route-operator":
		vxlanConfig, err := liqonet.ReadVxlanNetConfig(defaultConfig)
		if err != nil {
			klog.Errorf("an error occurred while getting the vxlan network configuration: %s", err)
		}
		vxlanPort, err := strconv.Atoi(vxlanConfig.Port)
		if err != nil {
			klog.Errorf("unable to convert vxlan port %s to string", vxlanConfig.Port)
		}
		err = liqonet.CreateVxLANInterface(clientset, vxlanConfig)
		if err != nil {
			klog.Errorf("unable to create vxlan interface: %s", err)
		}
		//Enable loose mode reverse path filtering on the vxlan interfaces
		err = liqonet.Enable_rp_filter()
		if err != nil {
			klog.Errorf("an error occurred while enabling loose mode reverse path filtering: %s", err)
			os.Exit(3)
		}
		isGatewayNode, err := liqonet.IsGatewayNode(clientset)
		if err != nil {
			klog.Errorf("an error occurred while checking if the node is the GatewayNode: %s", err)
			os.Exit(2)
		}
		//get node name
		nodeName, err := liqonet.GetNodeName()
		if err != nil {
			klog.Errorf("unable to get node nome: %s", err)
			os.Exit(4)
		}
		gatewayVxlanIP, err := liqonet.GetGatewayVxlanIP(clientset, vxlanConfig)
		if err != nil {
			klog.Errorf("unable to build gateway vxlanIP: %s", err)
			os.Exit(5)
		}
		ipt, err := iptables.New()
		if err != nil {
			klog.Errorf("unable to initialize iptables, check if the binaries are present in the sysetm: %s", err)
			os.Exit(6)
		}
		r := &liqonetOperators.RouteController{
			Client:                             mgr.GetClient(),
			Scheme:                             mgr.GetScheme(),
			Recorder:                           mgr.GetEventRecorderFor(strings.Join([]string{"route-OP", nodeName}, "-")),
			ClientSet:                          clientset,
			IsGateway:                          isGatewayNode,
			VxlanNetwork:                       vxlanConfig.Network,
			VxlanIfaceName:                     vxlanConfig.DeviceName,
			VxlanPort:                          vxlanPort,
			IPTablesRuleSpecsReferencingChains: make(map[string]liqonet.IPtableRule),
			IPTablesChains:                     make(map[string]liqonet.IPTableChain),
			RoutesPerRemoteCluster:             make(map[string]netlink.Route),
			NodeName:                           nodeName,
			GatewayVxlanIP:                     gatewayVxlanIP,
			RetryTimeout:                       30 * time.Second,
			IPtables:                           ipt,
			NetLink:                            &liqonet.RouteManager{},
			Configured:                         make(chan bool, 1),
		}
		r.WatchConfiguration(config, &clusterConfig.GroupVersion)
		if !r.IsConfigured {
			<-r.Configured
			r.IsConfigured = true
			klog.Infof("route-operator configured with podCIDR %s", r.ClusterPodCIDR)
		}
		//this go routing ensures that the general chains and rulespecs for LIQO exist and are
		//at the first position
		quit := make(chan struct{})
		go func() {
			for {
				if err := r.CreateAndEnsureIPTablesChains(); err != nil {
					klog.Error(err)
				}
				select {
				case <-quit:
					klog.Infof("stopping go routing that ensure liqo iptables rules")
					return
				case <-time.After(liqonetOperators.ResyncPeriod):
				}
			}
		}()
		if err = r.SetupWithManager(mgr); err != nil {
			klog.Errorf("unable to setup controller: %s", err)
			os.Exit(1)
		}
		klog.Info("Starting manager as Route-Operator")
		if err := mgr.Start(r.SetupSignalHandlerForRouteOperator(quit, waitCleanUp)); err != nil {
			klog.Errorf("unable to start controller: %s", err)
			os.Exit(1)
		}
		<-waitCleanUp

	case "tunnel-operator":
		r := &liqonetOperators.TunnelController{
			Client:                       mgr.GetClient(),
			Scheme:                       mgr.GetScheme(),
			Recorder:                     mgr.GetEventRecorderFor("tunnel-operator"),
			TunnelIFacesPerRemoteCluster: make(map[string]int),
		}
		if err = r.SetupWithManager(mgr); err != nil {
			klog.Errorf("unable to setup controller: %s", err)
			os.Exit(1)
		}
		klog.Info("Starting manager as Tunnel-Operator")
		if err := mgr.Start(r.SetupSignalHandlerForTunnelOperator()); err != nil {
			klog.Errorf("unable to start controller: %s", err)
			os.Exit(1)
		}

	case "tunnelEndpointCreator-operator":

		//get IP of gatewayNode
		nodeList, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: "net.liqo.io/gateway=true",
		})
		if err != nil {
			klog.Errorf("an error occurred while getting nodes %s", err)
			os.Exit(-1)
		}
		if len(nodeList.Items) != 1 {
			klog.Errorf("no node or multiple nodes found with label \"net.liqo.io/gateway=true\"")
			os.Exit(-1)
		}
		gatewayIP := nodeList.Items[0].Status.Addresses[0].Address
		//creating dynamic client
		dynClient := dynamic.NewForConfigOrDie(mgr.GetConfig())
		//creating dynamicSharedInformerFactory
		dynFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, liqonetOperators.ResyncPeriod)
		r := &liqonetOperators.TunnelEndpointCreator{
			Client:                     mgr.GetClient(),
			Scheme:                     mgr.GetScheme(),
			DynClient:                  dynClient,
			DynFactory:                 dynFactory,
			GatewayIP:                  gatewayIP,
			ReservedSubnets:            make(map[string]*net.IPNet),
			Configured:                 make(chan bool, 1),
			ForeignClusterStartWatcher: make(chan bool, 1),
			ForeignClusterStopWatcher:  make(chan struct{}),
			IPManager: liqonet.IpManager{
				UsedSubnets:        make(map[string]*net.IPNet),
				FreeSubnets:        make(map[string]*net.IPNet),
				SubnetPerCluster:   make(map[string]*net.IPNet),
				ConflictingSubnets: make(map[string]*net.IPNet),
			},
			RetryTimeout: 30 * time.Second,
		}
		//starting the watchers
		go r.Watcher(r.DynFactory, liqonetOperators.ForeignClusterGVR, cache.ResourceEventHandlerFuncs{
			AddFunc:    r.ForeignClusterHandlerAdd,
			UpdateFunc: r.ForeignClusterHandlerUpdate,
			DeleteFunc: r.ForeignClusterHandlerDelete,
		}, r.ForeignClusterStartWatcher, r.ForeignClusterStopWatcher)
		//starting configuration watcher
		r.WatchConfiguration(config, &clusterConfig.GroupVersion)
		if err = r.SetupWithManager(mgr); err != nil {
			klog.Errorf("unable to create controller controller TunnelEndpointCreator: %s", err)
			os.Exit(1)
		}
		klog.Info("starting manager as tunnelEndpointCreator-operator")
		if err := mgr.Start(r.SetupSignalHandlerForTunEndCreator()); err != nil {
			klog.Errorf("an error occurred while starting manager: %s", err)
			os.Exit(1)
		}
	}

}
