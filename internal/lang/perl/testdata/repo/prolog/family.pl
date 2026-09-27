% family relations
:- module(family, [parent/2]).
parent(tom, bob).
ancestor(X, Y) :- parent(X, Y).
