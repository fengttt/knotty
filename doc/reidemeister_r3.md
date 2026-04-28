Implement the R3 move.

First user will use the lasso tool to define an area.  An R3 move can be applied when ,
      1. There are arcs going into the area.  Following each arc, it will go out of the area.  This is called
         a strand.  That is, there are 3 strands.  Thus the diagram has 6 intersection points with the lasso boundary.
      2. There are two crossing points on each of the strand.
      3. There is one strand, that the two crossing points on the strand are either both over crossing or both under crossing.
         Call this strand the movable strand.  Denote the two crossing points on the movable strand A and B, and the crossing 
         point NOT on the movable strand C. 

Now, each strand at this moment can be denoted by the crossing points on it.  That is, the movable strand is AB, the 
other two strands are AC and BC.   Fix C, and move A along strand AC to the otherside of C, call the new crossing point A'.
Fix C, move B along strand BC to the other size of C.  Call the new crossing point B'.

Draw strand A'C, replacing original strand AC.   Note that the curve of strand A'C to AC should not move, all we need to do 
is keeping the over or under flag at point C, and A to A'.   Draw strand B'C in the same way replacing BC.

Finally, draw strand A'B' to replace strand AB.  Curve of strand A'B' is different from stand AB.  Connect the intersection 
of strand AB with lasso boundary to A, with a new curve to A'.   A' to B'.  Then B' to out going intersection of strand AB 
with the lasso bounday.  

Update Diagram, replacing crossing point A with A', B with B'.  And update all the arcs of the diagram to use A', B'.  
Especially note that for the two non-movable strands, the knot diagram actually changed from arc (X, A), (A, C), (C, Y) 
to (X, C), (C, A'), (A' Y), as we moved A to A' on the other side of C along this strand. 

