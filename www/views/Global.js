var GlobalView = function () {
    this.html = 'views/Global.html'
    this.stateBinds = ['authLevel', 'editMode']
    this.isActive = false
    this.data = {host: location.host}
    // this.refreshTid = 0
}

GlobalView.prototype.onShow = function () {
    var that = this
    // actions.call('global.list')
    http.get('/global').then(function (data) {
        var vars = []
        for (var k in data.vars) {
            vars.push({name: k, value: data.vars[k]})
        }
        var _vars = CP(vars)
        vars.push({})
        that.setData({
            vars: vars,
            _vars: _vars,
            publicKey: data.publicKey,
            sskeyToken: data.sskeyToken,
        })
    })

    http.get('/caches').then(function (data) {
        that.setData({caches: data})
    })

    this.isActive = true
    states.state.currentModule = this
    // this.refreshTid = setInterval(this.refreshStatus, 5000, this)
}

GlobalView.prototype.canHide = function () {
    if (this.data.changed) {
        if (!confirm('Data has changed, do you want drop them?')) return false
        this.data.changed = false
    }
    return true
}

GlobalView.prototype.onHide = function () {
    this.isActive = false
}

GlobalView.prototype.save = function () {
    var vars = {}
    for (var k in this.data.vars) {
        var v = this.data.vars[k]
        if (!v.name) {
            continue
        }
        vars[v.name.trim()] = v.value
    }

    var that = this
    http.post('/global', {vars: vars, sskeyToken: this.data.sskeyToken}).then(function () {
        that.setData({changed: false})
        that.onShow()
    }).catch(function (reason) {
        alert('Save global has error: ' + reason)
    })
}

GlobalView.prototype.check = function (event, type, idx) {
    var oldList = this.data['_' + type]
    var list = this.data[type]
    if ((idx < oldList.length && JSON.stringify(list[idx]) !== JSON.stringify(oldList[idx])) ||
        (idx >= oldList.length && list[idx].name)) {
        list[idx].changed = true
        if (this.data.changed !== true) {
            this.data.changed = true
        }
        // tpl.refresh(event.target.parentElement.parentElement, {index: idx, item: list[idx]})
        this.refreshView()
    }
    if (idx == list.length - 1) {
        list.push({})
        this.refreshView()
    }
}


GlobalView.prototype.clean = function (cacheName) {
    if (!confirm('sure to remove caches ' + cacheName + ' ?')) {
        return
    }

    var that = this
    http.delete('/cache/' + cacheName).then(function (data) {
        if (data === true) {
            http.get('/caches').then(function (data) {
                that.setData({caches: data})
            })
            alert(cacheName + ' cleaned')
        }
    })
}

function makeSize(size) {
    if (size < 1024) {
        return size
    } else if (size < 1024 * 1024) {
        return (size / 1024).toFixed(2) + 'K'
    } else if (size < 1024 * 1024 * 1024) {
        return (size / 1024 / 1024).toFixed(2) + 'M'
    }
}