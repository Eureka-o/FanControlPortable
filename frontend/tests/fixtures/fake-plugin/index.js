(function () {
  var host = window.FanControlPluginHost;
  var React = host.React;

  function FakePluginPage(props) {
    var plugin = props.host;
    var ui = plugin.ui;
    var FanIcon = plugin.icons.fan;
    var state = React.useState('ready');
    var result = state[0];
    var setResult = state[1];

    React.useEffect(function () {
      return plugin.subscribe('status', function (payload) {
        setResult(typeof payload === 'string' ? payload : JSON.stringify(payload));
      });
    }, [plugin]);

    return React.createElement('section', { className: 'fake-plugin-page', 'data-testid': 'fake-plugin-page' },
      React.createElement('header', { className: 'fake-plugin-header' },
        React.createElement('span', { className: 'fake-plugin-icon' }, React.createElement(FanIcon, { size: 20 })),
        React.createElement('div', null,
          React.createElement('h1', null, 'Official Plugin Test'),
          React.createElement('p', null, 'Host API v' + window.FanControlPluginHost.version)
        )
      ),
      React.createElement(ui.Card, { className: 'fake-plugin-card' },
        React.createElement('p', { 'data-testid': 'fake-plugin-result' }, result),
        React.createElement(ui.Button, {
          type: 'button',
          size: 'sm',
          onClick: function () {
            plugin.invoke('ping', { source: 'ui' }).then(function (value) {
              setResult(value.message);
            });
          }
        }, 'Invoke backend')
      )
    );
  }

  host.registerPage({ id: 'control', component: FakePluginPage });
})();
