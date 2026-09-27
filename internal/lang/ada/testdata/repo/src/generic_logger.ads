generic
   Name : String;
   with procedure Put (S : String) is <>;
package Generic_Logger is
   procedure Info (Msg : String);
end Generic_Logger;
