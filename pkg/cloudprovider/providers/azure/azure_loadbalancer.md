# Azure LoadBalancer
The way azure define LoadBalancer is different with GCE or AWS. Azure's LB can have multiple frontend IP refs. The GCE and AWS can only allow one, if you want more, you better to have another LB. Because of the fact, Public IP is not part of the LB in Azure. NSG is not part of LB in Azure as well. However, you cannot delete them in parallel, Public IP can only be delete after LB's frontend IP ref is removed. 

For different Azure Resources, such as LB, Public IP, NSG. They are the same tier azure resourceS. We need to make sure there is no connection in their own ensure loops. In another words, They would be eventually reconciled regardless of other resources' state. They should only depends on service state.

And also, For Azure, we cannot afford to have more than 1 worker of service_controller. Because, different services could operate on the same LB, concurrent execution could result in conflict or unexpected result. For AWS and GCE, they apparently doesn't have the problem, they use one LB per service, no such conflict.

## Introduce Functions

  - reconcileLoadBalancer(lb network.LoadBalancer, clusterName string, service *v1.Service, nodes []*v1.Node, wantLB bool) (network.LoadBalancer, error)
    - Go through lb's properties, update based on wantLB
    - If any change on the lb, no matter if the lb exists or not
      - Call az cloud to CreateOrUpdate on this lb, or Delete if nothing left
    - return lb, err

  - reconcileSecurityGroup(sg network.SecurityGroup, clusterName string, service *v1.Service, wantLb bool) (network.SecurityGroup, error)
    - Go though NSG' properties, update based on wantLB
    - If any change on the NSG, (the NSG should always exists)
      - Call az cloud to CreateOrUpdate on this NSG
    - return sg, err

  - reconcilePublicIP(pipName string, clusterName string, service *v1.Service, wantLB bool) (error)
    - if wantLB and external LB, 
      - ensure Azure Public IP resource is there
      - when we ensure Public IP, it needs to be both Name and Tag match with the convention
        - remove dangling Public IP that could have Name or Tag match with the service, but not both
    - else, ensure Azure Public IP resource is not there

## Define interface behaviors

### GetLoadBalancer
  - Get LoadBalancer status, return status, error
    - If not exist, ensure it is there

### EnsureLoadBalancer
  - Reconcile LB's related but not owned resources, such as Public IP, NSG rules
    - Call reconcileSecurityGroup(sg, clusterName, service, true)
    - Call reconcilePublicIP(pipName, cluster, service, true)
  - Reconcile LB's related and owned resources, such as FrontEndIPConfig, Rules, Probe.
    - Call reconcileLoadBalancer(lb, clusterName, service, nodes, true)

### UpdateLoadBalancer
  - Has no difference with EnsureLoadBalancer

### EnsureLoadBalancerDeleted
  - Reconcile NSG first, before reconcile LB, because SG need LB to be there
    - Call reconcileSecurityGroup(sg, clusterName, service, false)
  - Reconcile LB's related and owned resources, such as FrontEndIPConfig, Rules, Probe.
    - Call reconcileLoadBalancer(lb, clusterName, service, nodes, false)
  - Reconcile LB's related but not owned resources, such as Public IP
    - Call reconcilePublicIP(pipName, cluster, service, false)