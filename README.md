# SAP-challenge
### Algorithm:
As described in the question, each node must have both right & left child and also first & last nodes must be connected.
For example: x1 -> x2 -> x3 -> x4 -> x1.
Therefore, I decided to loop over the graph by running over one side only.
I was able to sum the correct amount of space once I reached the first node again.

Pros:
Visit each node once - O(n). 
No need to change node status.

Cons:
Lack of use of parallelism.
