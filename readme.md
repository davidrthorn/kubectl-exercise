# Kubernetes configMap controller exercise

To build and run the controller, all you need to do is set a path variable `KUBECONFIG` that points
to a kubernetes config file for your cluster.  

# Questions
_Actual phrasing has been omitted to preserve repo anonymity_

# Different URL content from 'curl' request

I realised when I read this question again having written the code that this was the cause of a bug that
I wrongly attributed to a lack of queueing. My controller does not ensure that the update
that triggered it was not the result of the update that it has just performed itself. Hence,
if the url returned a different response every time (e.g. a timestamp), the controller would
update the configMap forever in a loop.

It is a happy coincidence that the URL returns a limited set of responses, because at some point,
the same response is received twice in a row, and Kubernetes is happy that the desired state
matches the observed state. My tests pass because my mock http client always returns the same result.

I wrongly assumed that the lack of queueing in my controller was the issue, but I don't think it was.
Had I diagnosed this issue earlier, I might have had time to rewrite the code. To fix this, I would
need a way to confirm that the configMap is in the desired state, where this desired state was not
recalculated every time the controller fired. 

