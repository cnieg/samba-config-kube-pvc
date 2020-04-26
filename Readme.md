![Go](https://github.com/cnieg/samba-config-kube-pvc/workflows/Go/badge.svg)
# samba-config-kube-pvc

Controller to scrape nfs pvc information and generate samba config with limited access from Active Directory group

## How does it works

This go program connects with a kubeconfig file to a kubernetes cluster and scan each pvc which contains the annotations defined for the controller

It will get the some information on all pvcs filtered by the annotation as :
- Namespace
- PVC name
- Volume Name

With all of this informations and some parameters this will build a samba configuration derived from the template : [smb.conf.tmpl](./resources/template-samba-config/smb.conf.tmpl)

## Usage
Usage of /app/samba-config-kube-pvc:

```
  -annotationsToWatch string
        Annotations used to filter pvc list (default: cnieg.fr/samba-share) (default "cnieg.fr/samba-share")
  -defaultForceGroup string
        Group used by samba to edit file when a user edit files under the share (default: root) (default "root")
  -defaultForceUser string
        User used by samba to edit file when a user edit files under the share (default: root) (default "root")
  -defaultValidUsers string
        Groups autorized to access to the share, it can be locals user to the server or AD Group, list separated by comma (default: '')
  -dnsForwarder string
        IP of your dns server (default: '')
  -guestOk string
        Grant access to user anonymous, yes or no (default: no) (default "no")
  -inClusterConfig
        If your deployment is in kubernetes, to use the token in the pod for auth to kube (default: false)
  -kubeconfig string
        Path to your kubeconfig file for auth to kube (default: $HOME/.kube/config) (default "/.kube/config")
  -netbiosName string
        Name of your machine (default: hostname of your machine) (default "8f1e32552b69")
  -nfsMountPoint string
         Mount point on your server to your nfs share (mandatory)
  -period duration
        Watch period in seconds for list pvc (default: 30) (default 30ns)
  -realm string
        Realm of your domain (mandatory)
  -smbConfPath string
        Path to the smb config file (default: /etc/samba/smb.conf) (default "/etc/samba/smb.conf")
  -tmpDir string
        Working directory target for templating (default: ./tmp) (default "./tmp")
  -workgroup string
        Workgroup name of your domain (default: workgroup) (default "WORKGROUP")
  -writable string
        Writable share for users, yes or no (default: no) (default "yes")
 ```

## Requirements


We preconize to deploy with docker 

You should have docker-ce installed on your system

```bash
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu bionic stable"
sudo apt update
sudo apt install docker-ce
```

You should have a kubernetes service account with the grant to list PVCS, if you don't have you can apply manifests under [resources/kubernetes-roles](./resources/kubernetes-roles)  :
```bash
kubectl apply -f kubernetes
```


you can generate your kubeconfig file with this snippet
```bash
# your server name goes here
server=yourkubeapiendpoint
# the name of the secret containing the service account token goes here
name=samba-config-kube-pvc
namespace=default

secret=$(kubectl get sa $name -n $namespace -o jsonpath='{.secrets[0].name}')
ca=$(kubectl get secret/$secret -n $namespace -o jsonpath='{.data.ca\.crt}')
token=$(kubectl get secret/$secret -n $namespace -o jsonpath='{.data.token}' | base64 --decode)

echo "
apiVersion: v1
kind: Config
clusters:
- name: default-cluster
  cluster:
    certificate-authority-data: ${ca}
    server: ${server}
contexts:
- name: default-context
  context:
    cluster: default-cluster
    namespace: $namespace
    user: default-user
current-context: default-context
users:
- name: default-user
  user:
    token: ${token}
" > kubeconfig
```
    

## Deployment on Ubuntu server with systemd

### Systemd daemon creation
Create the following content in /etc/systemd/system/samba-config-kube-pvc.service:

```ini
[Unit]
Description=samba-config-kube-pvc
After=docker.service
Requires=docker.service

[Service]
Type=simple
User=root
Group=root
LimitNOFILE=1024

Restart=on-failure
RestartSec=10
startLimitIntervalSec=60
TimeoutStartSec=0
Environment="IMAGE_TAG=v1.0.22"
ExecStartPre=-/usr/bin/docker stop samba-config-kube-pvc
ExecStartPre=-/usr/bin/docker rm samba-config-kube-pvc
ExecStartPre=/usr/bin/docker pull cnieg/samba-config-kube-pvc:${IMAGE_TAG}
ExecStart=/usr/bin/docker run --rm --name samba-config-kube-pvc -v /root/.kube:/.kube -v /etc/samba:/etc/samba cnieg/samba-config-kube-pvc:${IMAGE_TAG} -nfsMountPoint=/mnt/nfs-volumes-kube-server -realm=MYREALM -defaultValidUsers=GG_ADMINS,GG_USERS_WRITE

[Install]
WantedBy=multi-user.target
```

Activate the daemon at startup
```bash
systemctl enable samba-config-kube-pvc
```

## Override template

You can override the template file by mounting another docker volume 


You have to create a new directory for storing the template
```bash
mkdir /opt/template-samba-config
```

And update the daemon service /etc/systemd/system/samba-config-kube-pvc.service by adding a new docker volume :

```ini
ExecStart=/usr/bin/docker run --rm --name samba-config-kube-pvc -v /root/.kube:/root/.kube -v /etc/samba:/etc/samba -v /opt/template-samba-config:/app/resources/template-samba-config/  cnieg/go-samba-config-controller:${IMAGE_TAG} $ARGS
```

Reload the systemd conf
```bash
systemctl daemon-reload
```
You can now add your specific directive in /opt/template-samba-config/smb.conf.tmpl


## Update the controller

Edit the file /etc/systemd/system/samba-config-kube-pvc.service and update the value of IMAGE_TAG

To launch the new app:
```bash
systemctl daemon-reload
systemctl restart samba-config-kube-pvc
```

