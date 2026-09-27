--  Several units in one file, as gnatchop reads them.
package Tools is
   procedure Run;
end Tools;

package body Tools is
   procedure Run is
   begin
      null;
   end Run;
end Tools;

with Tools;
procedure Tools_Main is
begin
   Tools.Run;
end Tools_Main;
