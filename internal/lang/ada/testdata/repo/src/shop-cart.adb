with Ada.Text_IO;
with Shop.Log;

package body Shop.Cart is

   use type Ada.Containers.Count_Type;
   use all type Color;

   procedure Add (C : in out Cart; I : Item) is
      procedure Trace (Msg : String) is
      begin
         Ada.Text_IO.Put_Line (Msg);
      end Trace;
   begin
      if C.Items.Length > 100 then
         raise Constraint_Error with "with Fake.Raise;";
      elsif I.Price < 0.0 then
         return;
      end if;
      for K in 1 .. 3 loop
         case K is
            when 1 => Trace ("one");
            when others => null;
         end case;
      end loop;
      declare
         Tick : constant Character := ''';
      begin
         C.Items.Append (I);
      end;
      Shop.Log.Info ("added");
   end Add;

   function Total (C : Cart) return Natural is separate;

   function "+" (L, R : Item) return Money is (L.Price + R.Price);

   task body Worker is
   begin
      accept Start do
         null;
      end Start;
   end Worker;

   protected body Lock is
      procedure Seize is
      begin
         Held := True;
      end Seize;
   end Lock;

end Shop.Cart;
