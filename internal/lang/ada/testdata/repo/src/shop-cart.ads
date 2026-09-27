--  with Fake.Comment;
with Ada.Containers.Vectors, Ada.Strings.Unbounded;
limited with Shop.Orders;
private with Shop.Internal;
WITH gnatcoll.json;
use Ada.Strings.Unbounded;

package Shop.Cart is

   type Item is record
      Name  : Unbounded_String;
      Price : Money;
   end record;

   type Cart is tagged private;
   type Shape is limited interface;
   subtype Count is Natural range 0 .. 100;
   type Color is (Red, Green);

   Quote : constant Character := '"';
   Fake  : constant String := "with Fake.String;";
   Apos  : constant Character := ''';
   Semi  : constant Character := ';';
   First : constant Count := Count'First;
   Paren : constant Character := Character'('(');

   procedure Add (C : in out Cart; I : Item);
   function Total (C : Cart) return Natural;
   function "+" (L, R : Item) return Money;

   package Item_Vectors is new Ada.Containers.Vectors (Positive, Item);

   task type Worker is
      entry Start;
   end Worker;

   protected type Lock is
      procedure Seize;
   private
      Held : Boolean := False;
   end Lock;

private

   type Cart is tagged record
      Items : Item_Vectors.Vector;
   end record;

end Shop.Cart;
