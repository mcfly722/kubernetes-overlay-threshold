# kubernetes-overlay-threshold

Enables docker overlay disk MB threshold for pods.<br><br>
Module checks for each container ``/var/lib/docker/overlay2/<mountContainerId>/diff`` folder, and if it overcomes specified threshold, module restarts swollen pod with event to K8S cluster.<br>
