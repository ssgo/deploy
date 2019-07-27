var ContextView = function (name) {
    this.name = name
    this.data = {cname: name}
    this.html = 'views/Context.html'
    this.stateBinds = ['authLevel', 'editMode']
    this.isActive = false
    // this.building = false
}

ContextView.prototype.onShow = function () {
    var that = this
    http.get('/context/' + this.name).then(function (data) {
        that.setContextData(data)
    })

    this.isActive = true
    states.state.currentModule = this
}

ContextView.prototype.canHide = function () {
    if (this.data.changed) {
        if (!confirm('Data has changed, do you want drop them?')) return false
        this.data.changed = false
    }
    return true
}

ContextView.prototype.onHide = function () {
    this.isActive = false
}

ContextView.prototype.showTagWindow = function (projIndex) {
    var proj = this.data.projects[projIndex]
    this.setData({
        tagWindowShowing: true,
        buildProject: proj.name,
        buildProjectIndex: projIndex,
        tags: ['master'],
    })
    this.refreshTags(proj.name, false)
}

ContextView.prototype.hideTagWindow = function () {
    this.setData({
        tagWindowShowing: false
    })
}

ContextView.prototype.refreshTags = function (projectName, clean) {
    var that = this
    if(clean) {
        if(!confirm("Fix tags will remove project "+projectName+" and take some time to clone the code again.Are you sure to fix?")) {
            return
        }
    }
    optTagsText = (clean === true?"fixTags":"refreshTags")
    opsTags = document.getElementById(optTagsText)
    if(!opsTags) {
        http.get('/tags/' + this.name + '/' + projectName + '?clean=' + clean).then(function (data) {
            that.setData({tags: data})
        })
        return
    }
    waitText = "Please Wait"
    if(opsTags.innerText == waitText) {
        alert(waitText);
        return
    }
    var oText = opsTags.innerText
    opsTags.innerText = waitText
    http.get('/tags/' + this.name + '/' + projectName + '?clean=' + clean).then(function (data) {
        that.setData({tags: data})
        opsTags.innerText = oText
    })
}



ContextView.prototype.build = function (projIndex, tag) {
    var proj = this.data.projects[projIndex]
    var that = this
    if(tag == "master"){
        tag = prompt("Please enter a tag")
        if(!tag){
            return;
        }
        tag = tag.trim()
        if(tag.length==0) {
            return
        }
        tag = "_"+tag
    }
    var ws = new WebSocket('ws://' + location.host + '/ws-build/' + that.name + '/' + proj.name + '/' + tag + '?token=' + proj.token);
    ws.onmessage = function (evt) {
        that.output(evt.data)
    };
    // ws.onclose = function () {
    //     this.building = false
    // };
}

ContextView.prototype.output = function (str) {
    ta = this.$('.buildOutput')
    ta.append(str)
    if (ta.scrollTop + ta.clientHeight >= ta.scrollHeight * 0.9) {
        ta.scrollTop = ta.scrollHeight - ta.clientHeight;
    }
}

ContextView.prototype.showHistoryWindow = function (projectName) {
    this.setData({
        historyWindowShowing: true,
        currentProjectName: projectName,
        currentHistory: '',
        currentBuild: '',
        currentMonth: '',
        buildMonths: [],
        histories: [],
    })
    var that = this
    http.get('/histories/' + this.name + '/' + projectName).then(function (data) {
        that.setData({buildMonths: data})
        if (data.length > 0) {
            that.clickMonth(projectName, data[0])
            // that.showBuild(projectName, data[0])
        }
    })
}

ContextView.prototype.hideHistoryWindow = function () {
    this.setData({
        historyWindowShowing: false
    })
}


ContextView.prototype.toggleMonthMenu = function () {
    this.$('.dropdown').className = this.$('.dropdown').className === 'dropdown' ? 'dropdown open' : 'dropdown'
}

ContextView.prototype.clickMonth = function (projectName, month) {
    this.setData({currentMonth: month})
    this.$('.dropdown').className = 'dropdown'
    var that = this
    http.get('/histories/' + this.name + '/' + projectName + '/' + month).then(function (data) {
        that.setData({histories: data})
        if (data.length > 0) {
            that.showBuild(projectName, data[0])
        }
    })
}

ContextView.prototype.showBuild = function (projectName, build) {
    var savedPos = this.$('.historiesList').scrollTop
    this.setData({
        currentBuild: build
    }).then(function () {
        this.$('.historiesList').scrollTop = savedPos
    })

    var that = this
    http.get('/history/' + this.name + '/' + projectName + '/' + build).then(function (data) {
        that.setData({currentHistory: data}).then(function () {
            that.$('.historiesList').scrollTop = savedPos
        })
    })
}


ContextView.prototype.showCIWindow = function (proj, edit) {
    var that = this
    http.get('/ci/' + this.name + '/' + proj).then(function (data) {
        that.setData({
            ciWindowShowing: true,
            ciProject: proj,
            ci: data,
            ciReadonly: !edit,
        })
    })
}

ContextView.prototype.formatCI = function () {
    try {
        var yml = jsyaml.safeLoad(this.data.ci)
        var fixedYml = jsyaml.safeDump(yml)
    } catch (e) {
        alert(e)
        return
    }
    this.setData({ci: fixedYml})
}

ContextView.prototype.saveCI = function () {
    try {
        jsyaml.safeLoad(this.data.ci)
    } catch (e) {
        alert(e)
        return
    }

    var that = this
    http.post('/ci/' + this.name + '/' + this.data.ciProject, {ci: this.data.ci}).then(function (data) {
        if (data === true) {
            that.hideCIWindow()
        } else {
            alert('failed to save CI for ' + this.data.ciProject)
            that.hideCIWindow()
        }
    })
}

ContextView.prototype.hideCIWindow = function () {
    this.setData({
        ciWindowShowing: false
    })
}

ContextView.prototype.setContextData = function (data) {
    var vars = []
    for (var k in data.vars) {
        vars.push({name: k, value: data.vars[k]})
    }

    var projects = []
    for (var k in data.projects) {
        data.projects[k].name = k
        projects.push(data.projects[k])
    }

    var _vars = CP(vars)
    var _projects = CP(projects)

    vars.push({})
    projects.push({})

    this.setData({
        name: data.name,
        desc: data.desc,
        token: data.token,
        vars: vars,
        projects: projects,
        _vars: _vars,
        _projects: _projects,
    })
}

ContextView.prototype.save = function () {
    var projects = {}
    for (var k in this.data.projects) {
        var v = this.data.projects[k]
        if (!v.name) {
            continue
        }

        projects[v.name.trim()] = {
            desc: v.desc,
            token: v.token,
            tag: v.tag,
            repository: v.repository,
        }
    }

    var vars = {}
    for (var k in this.data.vars) {
        var v = this.data.vars[k]
        if (!v.name) {
            continue
        }
        vars[v.name.trim()] = v.value
    }

    var that = this
    http.post('/context/' + this.name.trim(), {
        desc: this.data.desc,
        projects: projects,
        vars: vars,
        token: this.data.token,
    }).then(function (data) {
        if (data) {
            that.setData({changed: false})
            that.onShow()
        } else {
            alert('Save context has failed')
        }
    }).catch(function (reason) {
        alert('Save context has error: ' + reason)
    })
}

ContextView.prototype.remove = function () {
    if (prompt('Please enter the context name to conform for remove') === this.name) {
        var that = this
        http.delete('/context/' + this.name).then(function (data) {
            states.set('contextChanged', that.name)
            that.setData({changed: false})
            route.go('/deploy/global')
        }).catch(function (reason) {
            alert('Remove context has error: ' + reason)
        })
    } else {
        alert('Context name is not match')
    }
}

ContextView.prototype.check = function (event, type, idx) {
    var oldList = this.data['_' + type]
    var list = this.data[type]
    var changed = false
    if ((idx < oldList.length && JSON.stringify(list[idx]) !== JSON.stringify(oldList[idx])) ||
        (idx >= oldList.length && list[idx].name)) {
        list[idx].changed = true
        if (this.data.changed !== true) {
            this.data.changed = true
        }
        // tpl.refresh(event.target.parentElement.parentElement, {index: idx, item: list[idx]})
        changed = true
    }
    if (idx === list.length - 1) {
        list.push({})
        changed = true
    }

    if (changed === true) {
        this.refreshView()
    }
}
