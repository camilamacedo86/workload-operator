# workload-operator

This operator is a solution to automatically register newly built workload clusters with an ArgoCD instance running on the management cluster. The scenario tackled by this operator is:

- **Scenario:** We have a management cluster built using Cluster API. We use this management cluster to build workload clusters for tenant consumption. Each workload cluster is represented as a Cluster CRD on the management cluster, with an associated kubeconfig stored as a secret.
- **Problem:** When a new workload cluster is built, it has to be manually registered with the ArgoCD instance running on the management cluster. How can we automatically do this without manual intervention?

The workload operator leverages Kubebuilder to automatically detect the creation of new workload clusters and register them with the existing ArgoCD instance.

_PS.: Note that Kubebuilder and Operator-SDK have the same scaffolds. Operator-SDK simply has some extra features, such as Scorecard, and provides additional scaffolds to help integrate with the Operator Framework._

## Description

The workload-operator automates the process of registering new workload clusters with ArgoCD. 
It listens for the creation of new Cluster CRDs, retrieves the associated kubeconfig, and invokes the necessary ArgoCD commands to register the cluster. 
This replaces the manual steps typically required to register a cluster with ArgoCD 

### Design solution

#### Approach
- **API Representation**: Create an API to represent the registration with ArgoCD (`registers.argocd.workload.com`).
- **Cluster Scope**: The Operator is designed with cluster scope, monitoring Cluster Resources (`clusters.cluster.x-k8s.io`) across the whole cluster.
- **CR for Registration**: For each Workload Cluster, the operator will create a CR of the Register Kind, representing the registration with ArgoCD (relationship 1..1).
- **Status Conditions**: The Register CR is populated with status conditions, allowing us to determine if the registration was successful.
- **ArgoCD Communication**: The adopted approach for communicating with ArgoCD is through its API via HTTP requests. The API documentation can be found [here](https://cd.apps.argoproj.io/swagger-ui).
   - **Alternative Options**: Other alternatives include using the ArgoCD API client (available [here](https://github.com/argoproj/argo-cd/tree/master/pkg/apiclient)) or utilizing its binary.
- **Maintainability**: In order to ensure maintainability, an interface (ArgoAPIManager) was created to interact with the ArgoAPI.

#### Tests
- **Testing Framework**: The project uses Ginkgo and Omega, following the TBD style, in alignment with the frameworks adopted by Kubernetes SIG tools and frameworks.
- **Unit Testing**: Both the Controller and ArgoAPIManager are unit tested using ENV Tests from the controller runtime.
- **End-to-End Testing**: E2E tests have been created under [test/e2e](./test/e2e), utilizing kind with context to simulate the multi-cluster scenario.
- **Continuous Integration**: GitHub Actions can be configured to run tests against Pull Requests, ensuring consistent code quality.
- **Code Quality**: Good practices such as adopting Golint have been set up in this project, reinforcing coding standards.

#### Alternative Options for ArgoCD Communication

Other alternatives include using the ArgoCD API client (available [here](https://github.com/argoproj/argo-cd/tree/master/pkg/apiclient)) or utilizing its binary
as the following examples within pros and crons.

## Getting Started

You'll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) for a local cluster or run against a remote cluster.

### Pre-requirements to use this project

- [ArgoCD](https://argo-cd.readthedocs.io/en/stable/operator-manual/notifications/services/github/) >= 2.0.0 installed
- Make sure your user is authorized with cluster-admin permissions.
- [Docker](https://docs.docker.com/engine/install/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Go](https://go.dev/doc/install) >= `1.20`
- [Cluster API CRD](https://doc.crds.dev/github.com/kubernetes-sigs/cluster-api/cluster.x-k8s.io/Cluster/v1beta1@v1.5.0) must be applied on cluster 

### Running on the cluster

- 1. **Install required manifests:**

   ```sh
   make install
   ```
  
> NOTE: For development purpose if you have not the cluster with Cluster API registred you might want
> to run the target `make install-cluster-api` to test it out. If you try to start out the Operator
> in a cluster without the required CRDs applied then it will fail in the Manager initialization.

- 2. Build and push your image to the location specified by IMG:

   ```sh
   make docker-build docker-push IMG=<some-registry>/workload-operator:tag
   ```
   
**IMPORTANT:** This image ought to be published in the personal registry you specified. And it is required to 
have access to pull the image from the working environment. Make sure you have the proper permission to the registry if the above commands don’t work.

> NOTE: By using kind you can instead just call `make docker-build IMG=<some-registry>/workload-operator:tag` to build the
> image and then, load the image on the cluster (i.e. `kind load docker-image <some-registry>/workload-operator:tag`) so that
> you do not need to have it in registry with public access. 

- 3. Deploy the Operator to the cluster with image specified by IMG:

   ```sh
   make deploy IMG=<some-registry>/workload-operator:tag
   ```

- 4. Apply a Cluster API CR with the data of the Workload Cluster

Now, the Operator should be deployed and when a Cluster API CR be applied into a cluster
it should try to perform the registration of the Workload Cluster with Argo CD. For development
purposes you might want to run the Makefile target `make apply-mock` to test it out. 

## Followups
- Create the makefile targets that will apply smaples and configure the dev env
- Create GitHub to run e2e tests
- Address improvements and fixes in the code implementation


