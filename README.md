﻿# kubernetes-overlay-threshold

Enables docker overlay disk threshold for pods.
Each pod that overcome this threshold will be deleted with k8s event message.

Also there are threshold for number of files, because many small files can affect k8s node too.
