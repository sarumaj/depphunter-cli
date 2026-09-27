syscall::open:entry
/execname == "shop"/
{
	printf("%s\n", copyinstr(arg0));
}
