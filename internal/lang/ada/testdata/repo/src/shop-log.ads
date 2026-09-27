with Generic_Logger;
with Ada.Text_IO; use Ada.Text_IO;

package Shop.Log is new Generic_Logger (Name => "shop", Put => Put_Line);
