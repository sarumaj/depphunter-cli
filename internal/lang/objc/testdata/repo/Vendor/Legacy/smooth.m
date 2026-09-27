% SMOOTH  moving average of a signal
function y = smooth(x, n)
    y = filter(ones(1, n) / n, 1, x);
end
