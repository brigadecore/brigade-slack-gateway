if 'ENABLE_NGROK_EXTENSION' in os.environ and os.environ['ENABLE_NGROK_EXTENSION'] == '1':
  v1alpha1.extension_repo(
    name = 'default',
    url = 'https://github.com/tilt-dev/tilt-extensions'
  )
  v1alpha1.extension(name = 'ngrok', repo_name = 'default', repo_path = 'ngrok')

load('ext://min_k8s_version', 'min_k8s_version')
min_k8s_version('1.18.0')

trigger_mode(TRIGGER_MODE_MANUAL)

load('ext://namespace', 'namespace_create')
namespace_create('brigade-slack-gateway')
k8s_resource(
  new_name = 'namespace',
  objects = ['brigade-slack-gateway:namespace'],
  labels = ['brigade-slack-gateway']
)

k8s_resource(
  new_name = 'common',
  objects = [
    'brigade-slack-gateway:secret',
    'brigade-slack-gateway-config:secret'
  ],
  labels = ['brigade-slack-gateway']
)

docker_build(
  'brigadecore/brigade-slack-gateway-monitor', '.',
  dockerfile = 'monitor/Dockerfile',
  only = [
    'internal/',
    'monitor/'
    'go.mod',
    'go.sum'
  ],
  ignore = ['**/*_test.go']
)
k8s_resource(
  workload = 'brigade-slack-gateway-monitor',
  new_name = 'monitor',
  labels = ['brigade-slack-gateway']
)

docker_build(
  'brigadecore/brigade-slack-gateway-receiver', '.',
  dockerfile = 'receiver/Dockerfile',
  only = [
    'internal/',
    'receiver/',
    'go.mod',
    'go.sum'
  ],
  ignore = ['**/*_test.go']
)
k8s_resource(
  workload = 'brigade-slack-gateway-receiver',
  new_name = 'receiver',
  port_forwards = '31700:8080',
  labels = ['brigade-slack-gateway']
)

k8s_yaml(
  helm(
    './charts/brigade-slack-gateway',
    name = 'brigade-slack-gateway',
    namespace = 'brigade-slack-gateway',
    set = [
      'brigade.apiToken=' + os.environ['BRIGADE_API_TOKEN'],
      'receiver.tls.enabled=false'
    ]
  )
)
