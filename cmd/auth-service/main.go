package main

import (
	"flag"
	auth_service "github.com/liqotech/liqo/internal/auth-service"
	"k8s.io/klog"
	"os"
	"path/filepath"
)

func main() {
	klog.Info("Starting")

	var namespace string
	var kubeconfigPath string

	flag.StringVar(&namespace, "namespace", "default", "Namespace where your configs are stored.")
	flag.StringVar(&kubeconfigPath, "kubeconfigPath", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "For debug purpose, set path to local kubeconfig")
	flag.Parse()

	klog.Info("Namespace: ", namespace)

	authService, err := auth_service.NewAuthServiceCtrl(namespace, kubeconfigPath)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	if err = authService.Start(); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}
