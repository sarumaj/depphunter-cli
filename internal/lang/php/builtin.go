package php

import "strings"

// PHP's built-in classes, functions and constants come from the engine and its
// extensions, never from Composer (composer.json names them only as platform
// requirements: php, ext-*). They go to the php-std island, one package per
// extension, named as Composer names the extension without its "ext-".

// builtinClasses maps each extension to the global classes and interfaces it defines.
//
// Implements: REQ-PHP-006
var builtinClasses = map[string]string{
	// cSpell: disable
	"core": `stdClass Exception ErrorException Error TypeError ValueError ArithmeticError
		DivisionByZeroError ArgumentCountError AssertionError CompileError ParseError
		UnhandledMatchError Closure Generator ClosedGeneratorException WeakReference WeakMap
		Attribute ReturnTypeWillChange AllowDynamicProperties SensitiveParameter
		SensitiveParameterValue Override Deprecated Fiber FiberError Traversable
		IteratorAggregate Iterator ArrayAccess Countable Serializable Stringable UnitEnum
		BackedEnum InternalIterator Throwable __PHP_Incomplete_Class php_user_filter Directory
		RequestParseBodyException NoDiscard`,
	"date": `DateTime DateTimeImmutable DateTimeInterface DateTimeZone DateInterval DatePeriod
		DateError DateObjectError DateRangeError DateException DateInvalidTimeZoneException
		DateInvalidOperationException DateMalformedStringException
		DateMalformedIntervalStringException DateMalformedPeriodStringException`,
	"spl": `ArrayObject ArrayIterator RecursiveArrayIterator SplStack SplQueue
		SplDoublyLinkedList SplObjectStorage SplFixedArray SplHeap SplMinHeap SplMaxHeap
		SplPriorityQueue SplTempFileObject SplFileObject SplFileInfo DirectoryIterator
		FilesystemIterator RecursiveDirectoryIterator GlobIterator RecursiveIteratorIterator
		IteratorIterator FilterIterator CallbackFilterIterator RecursiveCallbackFilterIterator
		RecursiveFilterIterator ParentIterator LimitIterator CachingIterator
		RecursiveCachingIterator NoRewindIterator AppendIterator InfiniteIterator RegexIterator
		RecursiveRegexIterator EmptyIterator RecursiveTreeIterator MultipleIterator
		OuterIterator RecursiveIterator SeekableIterator SplObserver SplSubject LogicException
		BadFunctionCallException BadMethodCallException DomainException
		InvalidArgumentException LengthException OutOfRangeException RuntimeException
		OutOfBoundsException OverflowException RangeException UnderflowException
		UnexpectedValueException`,
	"json": `JsonSerializable JsonException`,
	"reflection": `Reflection ReflectionClass ReflectionObject ReflectionMethod
		ReflectionFunction ReflectionFunctionAbstract ReflectionProperty ReflectionParameter
		ReflectionNamedType ReflectionUnionType ReflectionIntersectionType ReflectionType
		ReflectionException ReflectionEnum ReflectionEnumBackedCase ReflectionEnumUnitCase
		ReflectionClassConstant ReflectionAttribute ReflectionExtension
		ReflectionZendExtension ReflectionGenerator ReflectionFiber ReflectionReference
		Reflector ReflectionConstant`,
	"pdo":     `PDO PDOStatement PDOException PDORow`,
	"mysqli":  `mysqli mysqli_result mysqli_stmt mysqli_driver mysqli_warning mysqli_sql_exception`,
	"sqlite3": `SQLite3 SQLite3Stmt SQLite3Result SQLite3Exception`,
	"dom": `DOMDocument DOMElement DOMNode DOMNodeList DOMXPath DOMAttr DOMText DOMComment
		DOMCdataSection DOMDocumentFragment DOMDocumentType DOMEntity DOMEntityReference
		DOMException DOMImplementation DOMNamedNodeMap DOMNotation DOMProcessingInstruction
		DOMCharacterData DOMParentNode DOMChildNode DOMNameSpaceNode`,
	"simplexml": `SimpleXMLElement SimpleXMLIterator`,
	"xmlreader": `XMLReader`,
	"xmlwriter": `XMLWriter`,
	"xml":       `XMLParser`,
	"xsl":       `XSLTProcessor`,
	"libxml":    `LibXMLError`,
	"intl": `Collator NumberFormatter Locale Normalizer MessageFormatter IntlDateFormatter
		IntlDatePatternGenerator ResourceBundle Spoofchecker Transliterator IntlCalendar
		IntlGregorianCalendar IntlTimeZone IntlBreakIterator IntlRuleBasedBreakIterator
		IntlCodePointBreakIterator IntlPartsIterator IntlIterator IntlException IntlChar
		UConverter`,
	"curl":      `CurlHandle CurlMultiHandle CurlShareHandle CURLFile CURLStringFile`,
	"openssl":   `OpenSSLCertificate OpenSSLCertificateSigningRequest OpenSSLAsymmetricKey`,
	"sodium":    `SodiumException`,
	"zip":       `ZipArchive`,
	"phar":      `Phar PharData PharFileInfo PharException`,
	"gd":        `GdImage GdFont`,
	"hash":      `HashContext`,
	"ffi":       `FFI`,
	"fileinfo":  `finfo`,
	"sockets":   `Socket AddressInfo`,
	"session":   `SessionHandler SessionHandlerInterface SessionIdInterface SessionUpdateTimestampHandlerInterface`,
	"soap":      `SoapClient SoapServer SoapFault SoapHeader SoapParam SoapVar`,
	"tokenizer": `PhpToken`,
	"zlib":      `DeflateContext InflateContext`,
	"gmp":       `GMP`,
	"shmop":     `Shmop`,
	"redis":     `Redis RedisCluster RedisException RedisClusterException RedisArray RedisSentinel`,
	"memcached": `Memcached MemcachedException`,
	"memcache":  `Memcache`,
	"imagick":   `Imagick ImagickDraw ImagickPixel ImagickPixelIterator ImagickException ImagickKernel`,
	"amqp":      `AMQPConnection AMQPChannel AMQPExchange AMQPQueue AMQPEnvelope AMQPException`,
	// cSpell: enable
}

// builtinNamespaces are the namespaces extensions define classes in.
var builtinNamespaces = map[string]string{
	// cSpell: disable
	`Random\`: "random", `FFI\`: "ffi", `Dom\`: "dom", `Pdo\`: "pdo", `PgSql\`: "pgsql",
	`LDAP\`: "ldap", `FTP\`: "ftp", `IMAP\`: "imap", `Dba\`: "dba", `Odbc\`: "odbc",
	`MongoDB\Driver\`: "mongodb", `MongoDB\BSON\`: "mongodb", `Swoole\`: "swoole",
	`Ds\`: "ds", `parallel\`: "parallel", `Pcntl\`: "pcntl", `BcMath\`: "bcmath",
	`Uri\`: "uri",
	// cSpell: enable
}

// builtinFunctionPrefixes attributes a global function to its extension by prefix,
// longest first; standardFunctions are the engine's and ext/standard's own, which
// follow no prefix.
var builtinFunctionPrefixes = []struct{ prefix, ext string }{
	// cSpell: disable
	{"array_", "standard"}, {"str_", "standard"}, {"ob_", "standard"},
	{"stream_", "standard"}, {"mb_", "mbstring"}, {"preg_", "pcre"}, {"json_", "json"},
	{"curl_", "curl"}, {"openssl_", "openssl"}, {"sodium_", "sodium"}, {"hash_", "hash"},
	{"ctype_", "ctype"}, {"iconv", "iconv"}, {"gz", "zlib"}, {"zlib_", "zlib"},
	{"date_", "date"}, {"timezone_", "date"}, {"mysqli_", "mysqli"}, {"pg_", "pgsql"},
	{"bc", "bcmath"}, {"gmp_", "gmp"}, {"grapheme_", "intl"}, {"idn_", "intl"},
	{"numfmt_", "intl"}, {"locale_", "intl"}, {"collator_", "intl"}, {"intl", "intl"},
	{"posix_", "posix"}, {"pcntl_", "pcntl"}, {"socket_", "sockets"},
	{"simplexml_", "simplexml"}, {"libxml_", "libxml"}, {"xml_", "xml"}, {"spl_", "spl"},
	{"iterator_", "spl"}, {"filter_", "filter"}, {"session_", "session"}, {"image", "gd"},
	{"exif_", "exif"}, {"ldap_", "ldap"}, {"ftp_", "ftp"}, {"finfo_", "fileinfo"},
	{"random_", "random"}, {"mt_", "random"}, {"opcache_", "opcache"}, {"apcu_", "apcu"},
	{"func_", "core"}, {"gc_", "core"}, {"zend_", "core"},
	// cSpell: enable
}

var standardFunctions = map[string]string{}

func init() {
	// cSpell: disable
	for ext, names := range map[string]string{
		"core": `strlen strcmp strncmp strcasecmp strncasecmp define defined constant
			class_exists interface_exists trait_exists enum_exists function_exists
			method_exists property_exists is_subclass_of is_a error_reporting trigger_error
			user_error set_error_handler restore_error_handler set_exception_handler
			restore_exception_handler each get_class get_parent_class get_object_vars
			get_class_methods get_class_vars get_called_class get_resource_type
			get_resource_id get_defined_vars get_defined_constants get_defined_functions
			get_loaded_extensions get_extension_funcs extension_loaded`,
		"spl": `class_implements class_parents class_uses`,
		"date": `date time mktime gmmktime checkdate strtotime gmdate idate getdate localtime
			strftime gmstrftime microtime hrtime`,
		"random":   `rand srand getrandmax lcg_value uniqid`,
		"fileinfo": `mime_content_type`,
		"standard": `count sizeof in_array implode explode join sprintf printf vsprintf vprintf
			fprintf sscanf number_format trim ltrim rtrim chop ucfirst lcfirst ucwords nl2br
			htmlspecialchars htmlentities html_entity_decode htmlspecialchars_decode strip_tags
			addslashes stripslashes wordwrap chunk_split substr substr_count substr_replace
			strpos stripos strrpos strripos strstr stristr strrchr strtolower strtoupper strtr
			strrev strval strcoll strspn strcspn strpbrk strnatcmp strnatcasecmp sort rsort
			usort uasort uksort ksort krsort asort arsort natsort natcasesort shuffle range
			compact extract min max abs ceil floor round intval floatval boolval settype
			gettype get_debug_type var_dump var_export print_r serialize unserialize md5
			sha1 crc32 base64_encode base64_decode bin2hex hex2bin urlencode urldecode
			rawurlencode rawurldecode http_build_query parse_str parse_url sleep usleep
			call_user_func call_user_func_array key current next prev reset end dirname
			basename pathinfo realpath getcwd chdir mkdir rmdir unlink rename copy touch
			fopen fclose fread fwrite fputs fgets fgetc feof fflush fseek ftell rewind
			ftruncate flock file file_exists file_get_contents file_put_contents fileperms
			filemtime filesize glob scandir opendir readdir closedir rewinddir tempnam
			tmpfile sys_get_temp_dir getenv putenv ini_get ini_set ini_restore set_time_limit
			header headers_sent header_remove setcookie setrawcookie http_response_code
			error_log version_compare phpversion php_sapi_name php_uname memory_get_usage
			memory_get_peak_usage ord chr pow sqrt pi fmod intdiv exp log log10 log2 sin cos
			tan deg2rad rad2deg hypot base_convert bindec decbin hexdec dechex octdec decoct
			levenshtein similar_text soundex metaphone escapeshellarg escapeshellcmd exec
			shell_exec system passthru proc_open proc_close popen pclose fsockopen
			gethostname gethostbyname gethostbynamel ip2long long2ip inet_pton inet_ntop
			key_exists iterator_count array assert highlight_string php_strip_whitespace
			register_shutdown_function uniqid lcfirst vfprintf money_format nl_langinfo
			setlocale localeconv quotemeta addcslashes stripcslashes str_contains
			password_hash password_verify password_needs_rehash crypt md5_file sha1_file
			fputcsv fgetcsv str_getcsv parse_ini_file parse_ini_string ignore_user_abort
			connection_status connection_aborted debug_backtrace debug_print_backtrace
			debug_zval_refcount array_walk usleep time_nanosleep chmod chown chgrp is_file
			umask clearstatcache disk_free_space readfile fpassthru symlink readlink link
			fnmatch mail flush getmypid getmyuid get_include_path set_include_path is_array
			is_string is_int is_integer is_long is_float is_double is_bool is_null is_numeric
			is_object is_callable is_iterable is_countable is_scalar is_resource is_dir
			is_link is_readable is_writable is_writeable is_executable is_uploaded_file
			is_nan is_finite is_infinite get_current_user get_headers get_meta_tags
			get_html_translation_table get_browser get_cfg_var`,
	} {
		for _, n := range strings.Fields(names) {
			standardFunctions[n] = ext
		}
	}
	// cSpell: enable
	lowered := map[string]string{}
	for ext, names := range builtinClasses {
		for _, n := range strings.Fields(names) {
			lowered[strings.ToLower(n)] = ext
		}
	}
	builtinClassIndex = lowered
}

// builtinClassIndex is builtinClasses by lower-case class name (PHP class names are
// case-insensitive).
var builtinClassIndex map[string]string

// builtin reports the extension a name PHP itself defines comes from.
//
// Implements: REQ-PHP-006
func builtin(fqn, kind string) (string, bool) {
	fqn = strings.TrimPrefix(fqn, `\`)
	if kind == kindClass {
		for prefix, ext := range builtinNamespaces {
			if len(fqn) > len(prefix) && strings.EqualFold(fqn[:len(prefix)], prefix) {
				return ext, true
			}
		}
		if strings.EqualFold(fqn, "FFI") {
			return "ffi", true
		}
	}
	if strings.Contains(fqn, `\`) {
		return "", false
	}
	low := strings.ToLower(fqn)
	switch kind {
	case kindClass:
		ext, ok := builtinClassIndex[low]
		return ext, ok
	case kindFunction:
		if ext, ok := standardFunctions[low]; ok {
			return ext, true
		}
		best, ext := 0, ""
		for _, p := range builtinFunctionPrefixes {
			if strings.HasPrefix(low, p.prefix) && len(p.prefix) > best {
				best, ext = len(p.prefix), p.ext
			}
		}
		return ext, ext != ""
	case kindConst:
		switch {
		case strings.HasPrefix(fqn, "JSON_"):
			return "json", true
		case strings.HasPrefix(fqn, "PREG_"):
			return "pcre", true
		case strings.HasPrefix(fqn, "PHP_"), strings.HasPrefix(fqn, "E_"), fqn == "DIRECTORY_SEPARATOR",
			fqn == "PATH_SEPARATOR", fqn == "PHP_EOL":
			return "core", true
		case strings.HasPrefix(fqn, "M_"), strings.HasPrefix(fqn, "SORT_"), strings.HasPrefix(fqn, "ENT_"):
			return "standard", true
		}
	}
	return "", false
}
