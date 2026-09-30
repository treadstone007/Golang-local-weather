# Golang-local-weather

This Go-based local weather application delivers fast, reliable forecasts from the National Weather Service at weather.gov. It can run on both personal computers and production servers. The code is stored and managed in a Github repo and can be shared and edited as needed based on approval.

The code can run on multiple servers in a cluster to allow scalability based on usage. 

Doing spikes, they’ll be nodes added to the cluster during peak time and spun down when request slow. 

The application will be monitored using Grafana and Prometheus, both the application layer and host layer will be monitored with an acceptable SLO. 

To secure the application the code will be stored in Github with access least privilege (PolP) rules applied. 

The nodes will be scanned and hardened to ensure industrial compliance. 

If external services are misbehaving, there will be an alert sent to the admins and a return message to the users as we work to resolve the technical issue. 

The application will be deployed via Github to ensure the code is built properly and current on each node. 
