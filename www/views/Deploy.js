var DeployView = {
    html: 'views/Deploy.html',
    stateBinds: ['authLevel', 'editMode'],

    getSubView: function (subName) {
        if (subName === 'global') {
            return new GlobalView()
        } else {
            return new ContextView(subName)
        }
    },

    onCreate: function () {
        this.stateRegisters = {contextChanged: [this, 'refreshContexts']}
    },

    onShow: function (path, nextPath) {
        if (nextPath) {
            this.data.nav = nextPath.name
        }
        route.bind('deploy.*', this)
        this.refreshContexts()
    },

    refreshContexts: function () {
        var that = this
        http.get('/contexts').then(function (data) {
            that.setData({contexts: data})
        })
    },

    onHide: function () {
        route.unbind('deploy.*', this)
    },

    onRoute: function (data) {
        this.setData({nav: data.last.name})
    },

    newContext: function () {
        var name = prompt('Enter a context name')
        if (name && /^[A-Za-z0-9_\.]+$/.test(name)) {
            this.data.contexts.push(name)
            http.post('/context/' + name, {desc: '', token: '', vars: {}, projects: {}}).then(function () {
                route.go('/deploy/' + name)
            }).catch(function (reason) {
                alert('Create context has error: ' + reason)
            })
        } else {
            alert('Context name is require [A-Za-z0-9_\\.]+')
        }
    },

}
