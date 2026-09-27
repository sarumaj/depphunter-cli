separate (Shop.Cart)
function Total (C : Cart) return Natural is
begin
   return Natural (C.Items.Length);
end Total;
