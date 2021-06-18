# Credential Rotator Operator

## Overview

The credential rotator operator was scaffolded via the [Operator SDK](https://sdk.operatorframework.io), and when the Custom Resource (CR) `CredentialRotator` is modified, it performs credential rotation as follows:

1. Creates service resource key for the backend service in question. In this case we are using a [Cloudant DB](https://en.wikipedia.org/wiki/Cloudant) deployed in [IBM Cloud](https://www.ibm.com/cloud).
2. Updates Secret with new resource key
3. Restarts Node.js [getting started application](https://github.com/IBM-Cloud/get-started-node) web app instances
4. Deletes previous resource key for [Cloudant DB](https://en.wikipedia.org/wiki/Cloudant) in [IBM Cloud](https://www.ibm.com/cloud)

## How to deploy

### Prerequisites

- Kubernetes cluster
- [IBM Cloud account](https://www.ibm.com/cloud)

### Steps

1. [Deploy web app and cloudant DB](#1-deploy-web-app-and-cloudant-db)
2. [Get the operator project](#2-get-the-operator-project)
3. [Compile, build, and push](#3-compile-build-and-push)
4. [Deploy the operator](#4-deploy-the-operator)
5. [Test and verify](#5-test-and-verify)

### 1. Deploy web app and cloudant DB

![Credential rotator operator](./images/credential_rotator_operator.png)

The application used in this tutorial to demonstrate the Credential Rotator Operator is the [Node.js](https://nodejs.org/en/) [getting started application](https://github.com/IBM-Cloud/get-started-node). This is a simple web application where you can add names which are stored in a backend Cloudant DB. The web app is deployed to a Kubernetes cluster and the [Cloudant DB service](https://cloud.ibm.com/catalog/services/cloudant) runs on [IBM Cloud](https://www.ibm.com/cloud). The web app connects to the DB using service credentials from the Cloudant service. These credentials are stored in a Secret on the cluster where the app is deployed so it can access them.

1. Initiate access to your Kubernetes cluster you want to deploy the web app and credential rotator operator on

2. Create a namespace (for example, `app-ns`) for deploying the web application into. For namespace `app-ns`:

```bash
$ kubectl create ns app-ns
```

3. Follow the steps in [Deploy to IBM Cloud Kubernetes Service](https://github.com/IBM-Cloud/get-started-node/blob/master/README-kubernetes.md) to deploy the web app to a Kubernetes cluster and cloudant DB to the IBM Cloud. **Remember to pass the namespace you created (for example `app-ns`) when deploying the app and running commands in the cluster for it.**

> Note: Do NOT follow the steps in [Clean Up](https://github.com/IBM-Cloud/get-started-node/blob/master/README-kubernetes.md#clean-up) as this will remove the web app deployed in the cluster.

4. Test that the deployed web app is working by adding a name and see if it is stored in the DB. The app is using the "Service Credentials" created during [Create a Cloudant Database](https://github.com/IBM-Cloud/get-started-node/blob/master/README-kubernetes.md#create-a-cloudant-database) step to access the Cloudant service. These credentials will be rotated by the operator.

### 2. Get the operator project

1. Check your Go version. This tutorial is tested with the following Go version:

    ```bash
    $ go version
    $ go version go1.16.5 darwin/amd64
    ```

2. Next, clone the operator GitHub repository.

    ```bash
    $ git clone git@github.com:hickeyma/credential-rotator.git
    $ cd credential-rotator
    ```

### 3. Compile, build, and push

Now you are ready to compile, build the image of the operator, and push the image to an image repository. You can use the image registry of your choice, but this tutorial uses [Docker Hub](https://hub.docker.com).

#### Compile the operator

To compile the code, run the following command in the terminal from your project root:

```bash
$ make install
```

#### Build and push image

**Note:** You will need to have an account to a image repository like Docker Hub to be able to push your operator image. Use `docker login` to log in.

1. To build the Docker image, run the following command. Note that you can also use the regular `docker build -t` command to build as well.

```bash
$ export IMG=docker.io/<username>/credential-rotator-operator:<version>
$ make docker-build IMG=$IMG
```

`<username>` is your Docker Hub (or Quay.io) username, and `<version>` is the
version of the operator image you will deploy. Note that each time you
make a change to operator code, it is good practice to increment the
version.

2. Push the Docker image to your registry using following command from your terminal:

```bash
$ make docker-push IMG=$IMG
```

### 4. Deploy the operator to your cluster

1. To deploy the operator, run the following command from your terminal:

    ```bash
    $ make deploy IMG=$IMG
    ```

    The output of the deployment should look like the following:

    ```bash
    ../go/src/github.com/hickeyma/credential-rotator-operator/bin/controller-gen "crd:trivialVersions=true,preserveUnknownFields=false" rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
    cd config/manager && ../go/src/github.com/hickeyma/credential-rotator-operator/bin/kustomize edit set image controller=docker.io/xxx/iam-credential-rotator-operator:latest
    ../go/src/github.com/hickeyma/credential-rotator-operator/bin/kustomize build config/default | kubectl apply -f -
    namespace/credential-rotator-operator-system created
    customresourcedefinition.apiextensions.k8s.io/credentialrotators.security.example.com configured
    serviceaccount/credential-rotator-operator-controller-manager created
    role.rbac.authorization.k8s.io/credential-rotator-operator-leader-election-role created
    clusterrole.rbac.authorization.k8s.io/credential-rotator-operator-manager-role created
    clusterrole.rbac.authorization.k8s.io/credential-rotator-operator-metrics-reader created
    clusterrole.rbac.authorization.k8s.io/credential-rotator-operator-proxy-role created
    rolebinding.rbac.authorization.k8s.io/credential-rotator-operator-leader-election-rolebinding created
    clusterrolebinding.rbac.authorization.k8s.io/credential-rotator-operator-manager-rolebinding created
    clusterrolebinding.rbac.authorization.k8s.io/credential-rotator-operator-proxy-rolebinding created
    configmap/credential-rotator-operator-manager-config created
    service/credential-rotator-operator-controller-manager-metrics-service created
    deployment.apps/credential-rotator-operator-controller-manager created
    ```

1. To make sure everything is working correctly, use the `kubectl get pods -n credential-rotator-operator-system` command.

    ```bash
    $ kubectl get pods -n credential-rotator-operator-system

    NAME                                                     READY   STATUS    RESTARTS   AGE
    credential-rotator-operator-controller-manager-54c5864f7b-znwws   2/2     Running   0          14s
    ```

This means the operator is up and running. Great job!

### 5. Test and verify

Now it is time to see if the operator can rotate the DB credentials and restart the web app instances. This means creating a CR instance.

> Note: If you have tested the web app with the DB outside of the operator (for example in [Deploy web app and cloudant DB](#1-deploy-web-app-and-cloudant-db)) then you need to remove the Secret that you created in the cluster. The operator will create a new Secret which is modifiable when the first CR is deployed. The Secret created outside of the operator is not compatible with the operator.

1. Update your custom resource, by modifying the `config/samples/security_v1alpha1_credentialrotator.yaml` file to look like the following:

    ```yaml
    apiVersion: security.example.com/v1alpha1
    kind: CredentialRotator
    metadata:
    name: credentialrotator-sample
    spec:
    userAPIKey:     "<IBM_USER_API_KEY>"
    serviceGUID:    "<CLOUDANT_SERVICE_GUID>"
    serviceURL:     "<CLOUDANT_SERVICE_ENDPOINT>"
    appName:        "my-app"
    appNameSpace:   "app-ns"
    ```

    where:
    - <IBM_USER_API_KEY>: User API key of the IBM Cloud account where the Cloudant service is running. Go to "Manage" -> "Access(IAM)" -> "API keys".
    - <CLOUDANT_SERVICE_GUID>: GUID of the Cloudant service instance. Click on service in "Resource List" and panel will appear on RHS which will contain "GUID" as a property.
    - <CLOUDANT_SERVICE_ENDPOINT>: Endpoint of the Cloudant service instance. Go to service instance full details and then "Manage"->"Overview" page.

2. Finally, create the custom resources using the following command:

    ```bash
    $ kubectl apply -f config/samples/security_v1alpha1_credentialrotator.yaml
    ```

#### Verify that credential rotation works

1. Go to web app site and you should be able to enter and save names to the DB.

2. The Web apps PODs should have been restarted.

3. The Cloudant "Service Credential" should have a new credential with a timestamp around time you created the CR.

> Note: You can remove any previous credentials that are not needed. The operator handles the credentials it creates, by replacing the previous credential with the new credential.

#### Cleanup

1. The `Makefile` part of generated project has a target called `undeploy` which deletes all the resources associated with the operator. It can be run as follows:

```bash
$ make undeploy
```

2. The app can be cleaned up by following the steps in [Clean Up](https://github.com/IBM-Cloud/get-started-node/blob/master/README-kubernetes.md#clean-up). **Remember to pass the namespace you created (for example `app-ns`) when running commands in the cluster for it.**

3. The Cloudant service can be deleted similar to [Deleting resource](https://cloud.ibm.com/docs/account?topic=account-delete-resource) in IBM Cloud.

## License

This code is licensed under the Apache Software License, Version 2.  Separate third party code objects invoked within this code are licensed by their respective providers pursuant to their own separate licenses. Contributions are subject to the [Developer Certificate of Origin, Version 1.1 (DCO)](https://developercertificate.org/) and the [Apache Software License, Version 2](https://www.apache.org/licenses/LICENSE-2.0.txt).

[Apache Software License (ASL) FAQ](https://www.apache.org/foundation/license-faq.html#WhatDoesItMEAN)
