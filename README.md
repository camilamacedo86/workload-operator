# workload-operator

## Introduction

The `workload-operator` provides an automated solution to a common problem in Kubernetes cluster management. When new workload clusters are built, they must be manually registered with ArgoCD. This operator offers an efficient, automated way to handle this registration, eliminating manual steps and enabling seamless integration within the Kubernetes and ArgoCD ecosystem.

## Description

The operator listens for the creation of new Cluster CRDs, retrieves the associated kubeconfig, and utilizes ArgoCD commands to automatically register the cluster. This programmatic approach replaces the manual steps typically involved, leading to a more efficient and consistent cluster registration process.

This project serves as a conceptual demonstration and is not a complete or workable solution in its current state. It is intended to illustrate a possible method to achieve the objective by proposing a specific design solution. Key features include:

- **Code Organization:** Clear and maintainable structure.
- **Reconciliation Logic:** Demonstrates the use of reconciliation with finalizers and status conditions.
- **Testing:** Provides examples of unit tests and outlines how end-to-end (e2e) tests could be implemented in this context.

### Use Case Scenario

This operator is a solution to automatically register newly built workload clusters with an ArgoCD instance running on the management cluster. The scenario tackled by this operator is:

- **Scenario:** We have a management cluster built using Cluster API. We use this management cluster to build workload clusters for tenant consumption. Each workload cluster is represented as a Cluster CRD on the management cluster, with an associated kubeconfig stored as a secret.
- **Problem:** When a new workload cluster is built, it has to be manually registered with the ArgoCD instance running on the management cluster. How can we automatically do this without manual intervention?

The workload operator leverages Kubebuilder to automatically detect the creation of new workload clusters and register them with the existing ArgoCD instance.

_PS.: Note that Kubebuilder and Operator-SDK have the same scaffolds. Operator-SDK simply has some extra features, such as Scorecard, and provides additional scaffolds to help integrate with the Operator Framework._

### Design Solution

#### Approach

- **API Representation**: Create an API to represent the registration with ArgoCD (`registers.argocd.workload.com`). ([More info]())
- **Cluster Scope**: The Operator is designed with cluster scope, monitoring Cluster Resources (`clusters.cluster.x-k8s.io`) across the whole cluster.
- **CR for Registration**: For each Workload Cluster, the operator will create a CR of the Register Kind, representing the registration with ArgoCD (relationship 1..1).
- **Status Conditions**: The Register CR is populated with status conditions, allowing us to determine if the registration was successful. Example:

```yaml
apiVersion: argocd.workload.com/v1beta1
kind: Register
metadata:
  name: my-cluster-registration
  namespace: my-namespace
spec:
  ...
status:
  conditions:
    - type: Available
      status: "True"
      reason: RegistrationComplete
      message: The cluster has been successfully registered with ArgoCD.
      lastTransitionTime: "2023-08-14T10:30:00Z"
```

- **ArgoCD Communication**: The adopted approach for communicating with ArgoCD is through its API via HTTP requests. The API documentation can be found [here](https://cd.apps.argoproj.io/swagger-ui).
- **Maintainability**: In order to ensure maintainability, an interface (ArgoAPIManager) was created to interact with the ArgoAPI.

#### Tests

- **Testing Framework**: The project uses Ginkgo and Omega, following the TBD style, in alignment with the frameworks adopted by Kubernetes SIG tools and frameworks.
- **Unit Testing**: Both the Controller and ArgoAPIManager are unit tested using ENV Tests from the controller runtime.
- **End-to-End Testing**: E2E tests have been created under [test/e2e](./test/e2e), utilizing kind with context to simulate the multi-cluster scenario.
- **Continuous Integration**: GitHub Actions can be configured to run tests against Pull Requests, ensuring consistent code quality.
- **Code Quality**: Good practices such as adopting Golint have been set up in this project, reinforcing coding standards.

#### Alternative Options for ArgoCD Communication

Other alternatives for communicating with ArgoCD include:

- **ArgoCD API Client:** This project does not use the ArgoCD API client due to its reliance on Kubernetes 1.24, necessitating downgrading various dependencies like the controller runtime to achieve compatibility. The cons include being constrained to older versions, limiting the ability to leverage newer features, and increased complexity in managing multiple Golang dependencies. This would inevitably reduce maintainability and hinder the project's evolution.
- **ArgoCD Binary:** Utilizing the binary would be feasible, provided that it is available on the cluster where the project is run. While implementation may be straightforward, the complexities of gathering, parsing, and error handling with the CLI output could significantly diminish long-term maintainability. However, it is still a great option to move forward.

#### Assumptions for Simplifying the Solution

- **Single Secret per Namespace:** Assumes only one kubeconfig secret exists per namespace representing the workload cluster. A more complex scenario might require watching and filtering secrets by labels or names.
- **Single ArgoCD Instance per Management Cluster:** This design assumes only one ArgoCD instance is installed on the management cluster. Multiple instances would necessitate a specification in the Register CR to define which ArgoCD instance is used. Additional logic would be needed to discover ArgoCD data for integration.
- **Static Kubeconfig Information:** The secret containing the kubeconfig information for the workload cluster is assumed to remain constant. Changes to this secret would require additional handling which in this case would require we either watch the secrets 
- **Unrestricted Network Access to Workload Clusters:** Assumes that the management cluster has full network access to all workload clusters, even those created outside its network. Consideration must be given to potential barriers such as network policies, firewalls, or other configurations that may restrict communication between the clusters.
- **Dependency on Specific Versions:** Compatibility with different versions of Kubernetes, Cluster API, or ArgoCD, which may have different APIs, behaviors, or requirements
- **Small number of Workload cluster:** Handling large numbers of clusters, namespaces, or secrets that may lead to performance issues.
  **Domain of Scope:** The Workload Operator is responsible for managing the Workload Clusters. As a result, a decision to adopt a multi-group layout has been made, providing the flexibility to have multiple APIs, controllers, and operations. Each of these will represent specific functionality, thereby organizing the overall management structure more effectively

#### Other further considerations

- **Security Considerations:** It would probably require to check how could we ensure security into communication between clusters as the secret with the kubeconfig info.
- **Handle Updates or Outages to ArgoCD:** Consideration for how to handle updates or outages to ArgoCD or the management cluster itself.
- **Network Latency:** Dealing with latency if the management and workload clusters are spread across different regions or cloud providers. We either need to
ensure timeouts and re-tries.
- **Disaster Recovery:** Planning for scenarios such as the accidental deletion of resources, clusters, or the operator itself.

### Conclusion

The `workload-operator` embodies modern practices in Kubernetes automation, offering a scalable and maintainable solution for automatic cluster registration with ArgoCD. By leveraging the official ArgoCD API, it minimizes technical debt and ensures alignment with ongoing ArgoCD development. This project serves as a foundational reference for similar automation efforts within the Kubernetes ecosystem and offers a sustainable path for development, aligning with modern practices and minimizing technical debt.

## Getting Started

To begin using the `workload-operator`, you'll need a Kubernetes cluster to run against. [KIND](https://sigs.k8s.io/kind) is an excellent choice for a local cluster, or you can run against a remote cluster.

### Pre-Requirements

- [ArgoCD](https://argo-cd.readthedocs.io/en/stable/operator-manual/notifications/services/github/) >= 2.0.0 installed
- Cluster-admin permissions for the current user
- [Docker](https://docs.docker.com/engine/install/) installed
- [kubectl](https://kubernetes.io/docs/tasks/tools/) installed
- [Go](https://go.dev/doc/install) version `1.20` or higher
- [Cluster API CRD](https://doc.crds.dev/github.com/kubernetes-sigs/cluster-api/cluster.x-k8s.io/Cluster/v1beta1@v1.5.0) applied on the cluster

### Running on the cluster

.1 - **Install required manifests:**

   ```sh
   make install
   ```
  
> NOTE: For development purposes, if your cluster does not have Cluster API registered, you may want to run the target make install-cluster-api to test it. If you try to start the Operator in a cluster without the required CRDs applied, it will fail in the Manager initialization.

.2 - Build and push your image to the location specified by IMG:

   ```sh
   make docker-build docker-push IMG=<some-registry>/workload-operator:tag
   ```

**IMPORTANT:** This image must be published in the personal registry you specified. Make sure that you have proper permissions for the registry, as you'll 
need access to pull the image from the working environment.

> NOTE: Using kind, you can instead call make docker-build IMG=<some-registry>/workload-operator:tag to build the image, 
> and then load the image on the cluster (i.e., kind load docker-image <some-registry>/workload-operator:tag) 
> so that you do not need it in a registry with public access.  

.3 - Deploy the Operator to the cluster with image specified by IMG:

   ```sh
   make deploy IMG=<some-registry>/workload-operator:tag
   ```

.4 - Apply a Cluster API CR with the data of the Workload Cluster

Now, the Operator should be deployed, and when a Cluster API CR is applied to a cluster, 
it should attempt to perform the registration of the Workload Cluster with ArgoCD. 




