[![status-badge](https://ci.spikedhand.com/api/badges/15/status.svg)](https://ci.spikedhand.com/repos/15)


# UPS Taint

Reads UPS details from a [NUT](https://networkupstools.org/) server and automatically taint Kubernetes nodes with battery state to gracefully shed pods during a power outage. Intended to only augment and does not replace standard scripts to power down machines.  

- Taints nodes as `ups.spikedhand.com/status=on-battery:NoSchedule` while running on battery power.
- Taints nodes as `ups.spikedhand.com/status=low-battery:NoExecute` if the UPS reports low battery.
- Taints nodes as `ups.spikedhand.com/status=below-threshold:NoExecute` if the UPS is below 50% battery.
- Removes the taint once the UPS is online again regardless of the charge level.

## Why? 

- My homelab has a small UPS.
- I want prioritize services that keep the network running well as long as possible because I work from home.
- Nova Scotia winter causes frequent short outages.
- Completely shutting down or draining a node for a usually short outage is overkill. 
- Many home server applications are stateful and not designed for clusters. I'd prefer them to have shutdown well before a low battery requires them to be.  

## Deployment

- Ensure your NUT server is configured to be network accessible to the cluster.
- Label all nodes with the appropriate UPS name configured in NUT. IE: `kubectl label node ${NODES} ups.spikedhand.com/name=${UPS_NAME}`
- `kubectl create ns ups-taint`
- `kubectl -n ups-taint create -f manifest/role.yml`
- `kubectl -n ups-taint create -f manifest/deployment.yml`
- Create a secret named `nut-server` in the `ups-taint` namespace with username, password, and address. Alternatively update the deployment to provide the needed environment variables in another way. 

