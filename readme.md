# Kubernetes configMap controller exercise

To build and run the controller, all you need to do is set a path variable `KUBECONFIG` that points
to a kubernetes config file for your cluster.  

## Questions
_Actual phrasing has been omitted to preserve repo anonymity_

### Different URL content from 'curl' request

I realised when I read this question again having written the code that this exact problem had was
causing a bug for me. I wrongly attributed to a lack of queueing, which I decided I lacked the time
and understanding to implement in this task.

Regrettably, my controller does not ensure that the update that triggered it was not the result of the
update that it has just performed itself. Hence, if the url returned a different response every time
(e.g. a timestamp), the controller would update the configMap forever in a loop. By the time
I figured this out, it was too late to fix it.

It is a happy (unhappy?) coincidence that the URL returns a limited set of responses, because at some point,
the same response is received twice in a row, and Kubernetes is happy that the desired state
matches the observed state. My tests pass because my mock http client always returns the same result.

I wrongly assumed that the lack of queueing in my controller was the issue, but I don't think it was.
Had I diagnosed this issue earlier, I might have had time to rewrite the code. To fix this, I would
need a way to confirm that the configMap is in the desired state, where this desired state was not
recalculated every time the controller fired. I could also add a test wherein the mockClient generates
a random response or iterates through an array of responses, which would have exposed this bug.

### Spec and status

The distinction between spec and status captures the fundamental aim of the Kubernetes control plane,
which is to maintain the state of a system by acting on knowledge of how the system is now and how it should be.
The spec specifies the desired state of the system; the status describes its current state.

The spec is outlined in the configuration yaml, whereas status can only be changed via the API (I believe). This is because
the purpose of the config file is the _describe_ or _declare_ the desired state. It would make no sense
to include status as a section in this file because a) the file says _what_ you want, not how to make it happen;
and b) the desired state is independent of the current state. The current state has no bearing on what we
want the system to look like.

### Edge vs level triggering

In simple terms, edge triggering is a matter of responding to a change of state; level triggering is a matter of responding
to the state itself. Edge triggering cares _that something changed_; level triggering cares _what state something is in now_.

Kubernetes is level triggered, which is not suprising given that its entire approach is conceptualised in terms of
arriving at a state, rather than performing a series of tasks (although it has to do things to arrive at a state, obviously).
One advantage of level triggering is that it is more forgiving than edge triggering. If Kubernetes were edge triggered,
it would perform a kind of 'repair' action for each change in the system. For example, if a pod died, then
K8s would register an action to add one more pod. If during the creation of this pod, another were to die, then K8 would
register another action to create a pod, and so on. But suppose, for some reason, a change were not detected or registed.
The system would not automatically recover the desired state. Instead, it would continue to register _new_ changes,
but the mistake would persist. For example, if the state is '2 pods up', and then one dies without K8s registering it (for some reason),
then the actual state would now be '1 pod up'. If that remaining pod dies, and K8s is now seeing changes again, 
K8 would know to recreate _1 pod_. But the state would still be wrong because this action only pertains the the change it observed.
With level triggering, by contrast, K8s will derive the actions that it needs to perform from the actual state of the system.
So if something goes wrong with monitoring, K8s will still self-heal once that monitoring has been restored. It will not stop
trying until the state of '2 pods up' is restored.

Of course, a well designed edge triggering system could be built to mitigate such issues. For example, edge triggering systems
could periodically reconcile by observing the state of the system. There are advantages to such an approach: for one thing,
edge triggering can trigger the moment something changes, whereas level triggering systems might rely on periodic observation.
If the edge system waits to be told about a change, whereas the level one periodically asks for an update on the state, then
the level system might not be updated until a little while after the change occurred -- the change might have occurred just
after the last check, for example. Meanwhile the edge system was notified of the change immediately. Hence edge triggering
could conceivably be more reactive than level triggering. (In pracice, however, anything that listens for a notification
is on some kind of loop that checks periodically for notifications, so this is a somewhat misleading distinction.) In reality,
there is no reason that a level triggering system could use edge triggering to alert it to observe the state, hence hybrid systems
are definitely a possibility.

