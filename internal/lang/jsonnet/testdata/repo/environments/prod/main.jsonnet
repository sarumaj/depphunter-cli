// The production environment. import "fake-line.libsonnet" is a comment.
# import 'fake-hash.libsonnet' too
/* import "fake-block.libsonnet" */
local kausal = import 'ksonnet-util/kausal.libsonnet';
local util = import "github.com/grafana/jsonnet-libs/ksonnet-util/util.libsonnet";
local k = import 'github.com/jsonnet-libs/k8s-libsonnet/1.29/main.libsonnet';
local k8s = import 'k8s/main.libsonnet';
local doc = import 'github.com/jsonnet-libs/docsonnet/doc-util/main.libsonnet';
local node = import 'node-mixin/mixin.libsonnet';
local utils = import 'utils.libsonnet';
local params = import './params.libsonnet';
local shared = import 'shared/lib.libsonnet';
local secrets = import 'libs/secrets.libsonnet';
local branchy = import 'branchy/b.libsonnet';
local unknown = import 'github.com/unknown/thing/x.libsonnet';
local mystery = import 'mystery/x.libsonnet';
local missing = import 'missing.libsonnet';
local gone = import './gone.libsonnet';
local extra = import 'extra.libsonnet';
local fake = "import 'fake-string.libsonnet'";
local verbatim = @'import "fake-verbatim.libsonnet" ''quoted''';
local text = |||
  import "fake-text.libsonnet"
  local x = 1;
|||;
local labels(name) = utils.labels(name);
local mk = function(x) x;

{
  config: importstr 'files/config.yaml',
  logo: importbin 'files/logo.png',
  deployment: kausal.util.deployment('app', params.replicas),
  'quoted-name': text,
  hidden:: verbatim,
  plus+: {},
  inner: {
    nested: 1,
  },
  fn(x):: x,
  asFunction: function(y) y,
  local private = 1,
  [std.format('computed-%s', 'x')]: private,
  again: importstr 'files/config.yaml',
} + {
  extended: true,
  config+: {},
}
