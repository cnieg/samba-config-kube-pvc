package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/cnieg/samba-config-kube-pvc/util/file"
	"github.com/udhos/equalfile"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"log"
	"net"
	"os"
	"path/filepath"
	"text/template"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	//
	// Uncomment to load all auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth"
	//
	// Or uncomment to load specific auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/openstack"
)

func main() {
	hostname, err := os.Hostname()
	type SambaShare struct {
		Name       string
		Comment    string
		Path       string
		ForceUser  string
		ForceGroup string
		Writable   string
		GuestOk    string
		ValidUsers string
	}

	type SambaConfig struct {
		Workgroup      string
		Realm          string
		NetbiosName    string
		DnsForwarder   net.IP
		SambaShareList []SambaShare
	}

	if err != nil {
		panic(err)
	}

	var nfsMountPoint = flag.String("nfsMountPoint", "", " Mount point on your server to your nfs share (mandatory)")
	var realm = flag.String("realm", "", "Realm of your domain (mandatory)")
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "Path to your kubeconfig file for auth to kube (default: $HOME/.kube/config)")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "Path to your kubeconfig file for auth to kube (default: $HOME/.kube/config)")
	}
	var inClusterConfig = flag.Bool("inClusterConfig", false, "If your deployment is in kubernetes, to use the token in the pod for auth to kube (default: false)")
	var annotationsToWatch = flag.String("annotationsToWatch", "cnieg.fr/samba-share", "Annotations used to filter pvc list (default: cnieg.fr/samba-share)")
	var period = flag.Duration("period", 30, "Watch period in seconds for list pvc (default: 30)")
	var workgroup = flag.String("workgroup", "WORKGROUP", "Workgroup name of your domain (default: workgroup)")
	var netbiosName = flag.String("netbiosName", hostname, "Name of your machine (default: hostname of your machine)")
	var dnsForwarder = flag.String("dnsForwarder", "", "IP of your dns server (default: '')")

	var defaultForceUser = flag.String("defaultForceUser", "root", "User used by samba to edit file when a user edit files under the share (default: root)")
	var defaultForceGroup = flag.String("defaultForceGroup", "root", "Group used by samba to edit file when a user edit files under the share (default: root)")
	var writable = flag.String("writable", "yes", "Writable share for users, yes or no (default: no)")
	var guestOk = flag.String("guestOk", "no", "Grant access to user anonymous, yes or no (default: no)")
	var defaultValidUsers = flag.String("defaultValidUsers", "", "Groups autorized to access to the share, it can be locals user to the server or AD Group (default: ''")

	var smbConfPath = flag.String("smbConfPath", "/etc/samba/smb.conf", "Path to the smb config file (default: /etc/samba/smb.conf)")
	var tmpDir = flag.String("tmpDir", "./tmp", "Working directory target for templating (default: ./tmp)")
	flag.Parse()

	if *nfsMountPoint == "" || *realm == "" {
		flag.PrintDefaults()
		log.Fatal("Parameters nfsMountPoint, realm are mandatory")
	}

	if _, err := os.Stat(*tmpDir); os.IsNotExist(err) {
		os.Mkdir(*tmpDir, os.ModePerm)
	}

	sambaConfig := SambaConfig{
		Workgroup:    *workgroup,
		Realm:        *realm,
		NetbiosName:  *netbiosName,
		DnsForwarder: net.ParseIP(*dnsForwarder),
	}

	// use the current context in kubeconfig
	var config *rest.Config
	if *inClusterConfig {
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err.Error())
		}
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	for {

		pvcs, err := clientset.CoreV1().PersistentVolumeClaims("").List(context.TODO(), metav1.ListOptions{})
		var pvcsOperated []v1.PersistentVolumeClaim
		if err != nil {
			panic(err.Error())
		}
		fmt.Printf("There are %d pvcs in the cluster\n", len(pvcs.Items))
		if *annotationsToWatch != "" {
			for _, pvc := range pvcs.Items {
				if pvc.Annotations[*annotationsToWatch] == "true" && pvc.Status.Phase == "Bound" {
					pvcsOperated = append(pvcsOperated, pvc)
				}
			}
		}
		for _, pvc := range pvcsOperated {

			fmt.Printf("PVC: %s , namespace: %s\n", pvc.GetName(), pvc.Namespace)
			sambaConfig.SambaShareList = append(sambaConfig.SambaShareList, SambaShare{
				Name:       pvc.GetName() + "-" + pvc.Namespace,
				Comment:    pvc.GetName() + "-" + pvc.Namespace,
				Path:       filepath.Join(*nfsMountPoint, pvc.Namespace+"-"+pvc.GetName()+"-"+pvc.Spec.VolumeName),
				ForceUser:  *defaultForceUser,
				ForceGroup: *defaultForceGroup,
				Writable:   *writable,
				GuestOk:    *guestOk,
				ValidUsers: *defaultValidUsers,
			})

		}

		//Generate sambaConfig
		tpl, err := template.ParseFiles("./resources/template-samba-config/smb.conf.tmpl")
		smbConfFile, err := os.Create(filepath.Join(*tmpDir, "smb.conf.tmp"))
		if err != nil {
			fmt.Printf("create file: %s", err)
			return
		}

		tpl.Execute(smbConfFile, sambaConfig)
		smbConfFile.Close()
		sambaConfig.SambaShareList = nil

		//Check difference between file
		cmp := equalfile.New(nil, equalfile.Options{}) // compare using single mode
		fileEqual, err := cmp.CompareFile(filepath.Join(*tmpDir, "smb.conf.tmp"), *smbConfPath)

		if !fileEqual {
			fmt.Printf("Copy new configuration")
			err := file.MoveFile(filepath.Join(*tmpDir, "smb.conf.tmp"), *smbConfPath)
			if err != nil {
				panic(err)
			}
		}
		time.Sleep(*period * time.Second)
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
