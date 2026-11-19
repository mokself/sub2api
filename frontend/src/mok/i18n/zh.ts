export default {
  mok: {
    claudePlan: {
      column: 'Claude 套餐',
      refresh: '重新获取套餐',
      failed: '获取失败',
      requestFailed: '套餐加载失败，可点右侧按钮重试',
      updatedAt: '更新于 {time}',
      attemptedAt: '最近尝试 {time}',
      lastError: '最近一次获取失败：{message}',
      unsupportedScope: '授权缺少 user:profile 权限，重新授权后才能获取套餐',
      unsupportedOther: '该账号无法获取套餐',
      subscriptionStatus: '订阅状态 {status}'
    },
    affiliateTiers: {
      title: '阶梯返利',
      description:
        '邀请人的返利比例随累计推广额提高：累计推广额（含本笔充值）达到某一档后，这一笔充值整笔按该档比例返利。低于第一档时按上面的「全局返利比例」；设了专属比例的用户继续用专属比例，不走阶梯。不设档位时，所有人都按全局比例。',
      volumeHint:
        '累计推广额 = 邀请人本人和名下所有被邀请人的累计充值：已完成且未退款的在线支付订单的到账金额（不含充值赠送），加上已使用的余额兑换码。管理员直接加的余额不计入。金额单位与余额相同。',
      minAmountLabel: '累计推广额达到',
      rateLabel: '返利比例',
      addTier: '添加档位',
      removeTier: '删除这一档',
      empty: '还没有档位，所有邀请人都按全局返利比例返利。',
      maxTiers: '最多 {max} 个档位',
      save: '保存阶梯',
      saving: '保存中…',
      saved: '阶梯返利已保存',
      unsaved: '有未保存的修改',
      loadFailed: '阶梯返利设置加载失败',
      saveFailed: '阶梯返利保存失败',
      retry: '重试',
      errors: {
        minAmountInvalid: '门槛金额要大于 0',
        rateInvalid: '返利比例要在 0 到 100 之间',
        minAmountDuplicate: '和其他档位的门槛金额重复'
      },
      user: {
        title: '阶梯返利',
        currentRate: '当前返利比例',
        teamVolume: '累计推广额',
        teamVolumeHint: '你自己和你邀请的用户累计充值的到账金额（不含充值赠送，含兑换码）',
        nextTier: '再累计 {amount}，返利比例升到 {rate}%',
        topTier: '已达到最高档',
        ladder: '返利阶梯',
        baseRow: '累计不足 {amount}',
        tierRow: '累计达到 {amount}',
        current: '当前',
        usageMode: '消耗返利也按当前档位的比例计算；升档仍看累计充值。'
      }
    },
    affiliateMode: {
      title: '返利方式',
      description:
        '充值返利：被邀请人充值时按比例返利（原有方式）。消耗返利：充值不再返利，改为每小时结算一次，按被邀请人上一个小时的余额消耗返利，订阅套餐内的用量不算。两种方式都按「专属比例 → 阶梯 → 全局比例」确定比例，阶梯仍按累计充值定档；单人返利上限、返利有效期、冻结期照常生效。',
      recharge: '充值返利',
      usage: '消耗返利',
      since: '消耗返利开始于 {time}，从这一刻起的余额消耗参与结算。',
      until: '消耗返利已于 {time} 停止，在此之前的消耗仍会结算完。',
      settleHint: '每个整点过后约 5 分钟结算上一个小时，整点按服务器时区计算。',
      settledUntil: '已结算到 {time}。',
      confirmTitle: '切换返利方式',
      confirmToUsage: '切换为消耗返利后，充值不再产生返利，从现在起被邀请人的余额消耗每小时结算一次。确定切换吗？',
      confirmToRecharge: '切换回充值返利后，从现在起的消耗不再返利（之前的消耗仍会结算完），充值重新产生返利。确定切换吗？',
      confirm: '确定切换',
      cancel: '取消',
      switched: '返利方式已切换',
      switchFailed: '返利方式切换失败',
      loadFailed: '返利方式加载失败',
      retry: '重试',
      recentDays: '最近结算（按天汇总）',
      viewRecords: '查看明细',
      skippedPeriods: '其中 {count} 个小时返利总开关关闭，未返利',
      noDays: '还没有结算记录',
      columns: {
        day: '日期',
        invitees: '人数',
        usage: '消耗',
        rebate: '返利'
      }
    },
    affiliateUsage: {
      title: '消耗返利明细',
      adminHint: '每个被邀请人每个结算时段一条记录，最新的在前。时段一般是一个小时，切换返利方式的那个小时会短一些。',
      userHint: '你邀请的用户每个小时的余额消耗，和你因此得到的返利，最新的在前。',
      searchPlaceholder: '搜索邀请人或被邀请人的邮箱、用户名',
      search: '搜索',
      loadFailed: '明细加载失败',
      retry: '重试',
      empty: '还没有结算记录',
      noMatch: '没有匹配的记录',
      columns: {
        period: '时段',
        inviter: '邀请人',
        invitee: '被邀请人',
        usage: '消耗',
        rate: '比例',
        rebate: '返利'
      },
      user: {
        title: '消耗返利',
        usageOn: '被邀请用户使用余额消费后，你按返利比例获得返利，每小时结算一次。',
        usageOff: '现在按充值返利；这里保留之前按消耗结算的返利记录。',
        latest: '最近一笔：{period}，返利 {amount}',
        none: '还没有结算记录。',
        view: '查看明细（{count}）',
        history: '这里保留你之前的消耗返利记录。'
      }
    },
    affiliateScope: {
      title: '开启范围',
      description:
        '邀请返利可以对全部用户开启，也可以只对指定的用户开启。只对指定用户开启时，其他用户看不到邀请返利入口，他们的邀请码无效，名下被邀请人的充值和消耗也不给他们返利；已有的返利额度仍然可以转入余额。',
      all: '全部用户',
      selected: '仅指定用户',
      summarySelected: '现在只对 {count} 个用户开启。',
      summaryAll: '现在对全部用户开启。已指定 {count} 个用户，改为「仅指定用户」后只对他们开启。',
      howTo: '在「用户管理」里编辑用户，可以打开或关闭这个用户的邀请返利。',
      listTitle: '已指定的用户',
      listFailed: '用户列表加载失败',
      columns: {
        user: '用户',
        selectedAt: '开启时间'
      },
      remove: '关闭',
      removed: '已关闭 {user} 的邀请返利',
      removeFailed: '关闭失败',
      confirmTitle: '切换开启范围',
      confirmToSelected: '切换后，只有已指定的 {count} 个用户有邀请返利；其他用户的邀请码失效，名下被邀请人的充值和消耗不再给他们返利。确定切换吗？',
      confirmToAll: '切换后，邀请返利对全部用户开启。确定切换吗？',
      confirm: '确定切换',
      cancel: '取消',
      switched: '开启范围已切换',
      switchFailed: '开启范围切换失败',
      loadFailed: '开启范围加载失败',
      retry: '重试',
      user: {
        label: '邀请返利',
        hintOn: '已对该用户开启：能看到邀请返利页，邀请码有效，能拿返利。',
        hintOff: '未对该用户开启：邀请码无效，名下被邀请人的充值和消耗不给他返利。',
        hintScopeAll: '现在邀请返利对全部用户开启，这个开关暂时不起作用。在系统设置里把开启范围改为「仅指定用户」后，只有打开这个开关的用户才有邀请返利。',
        hintFeatureOff: '邀请返利总开关已关闭，这个开关暂时不起作用。',
        instant: '切换后立即生效，不需要点「更新」。',
        loadFailed: '邀请返利开关加载失败',
        saveFailed: '邀请返利开关保存失败',
        switchedOn: '已对该用户开启邀请返利',
        switchedOff: '已对该用户关闭邀请返利'
      },
      notice: {
        title: '邀请返利暂未对你的账号开启',
        body: '你的邀请码目前无效，你邀请的用户充值或消耗也不会给你带来新的返利。已有的返利额度仍然可以转入余额。'
      }
    },
    sms: {
      title: '短信通知',
      description:
        '通过腾讯云短信发送余额不足提醒、订阅到期提醒，以及发给管理员的运维告警和账号限额告警。短信和邮件相互独立：用户绑定了手机号就收短信，添加了通知邮箱就收邮件，两者都有就都收。充值成功、订阅购买成功和风控通知不发短信。',
      enabled: '启用短信通知',
      dryRun: '仅记录不发送',
      dryRunHint: '打开后短信只写入发送记录、不真正发出，不填腾讯云配置也能试用整个流程；验证码会显示在发送记录里。',
      provider: '腾讯云短信',
      providerHint: '在腾讯云短信控制台创建应用、申请签名和模板，审核通过后填到这里。SecretKey 保存后不再显示。',
      secretId: 'SecretId',
      secretKey: 'SecretKey',
      secretKeySet: '已设置，留空不修改',
      sdkAppId: 'SDK AppID',
      signName: '短信签名',
      signNamePlaceholder: '审核通过的签名内容',
      region: '地域',
      regions: {
        guangzhou: '华南地区（广州）',
        beijing: '华北地区（北京）',
        nanjing: '华东地区（南京）'
      },
      templates: '短信模板',
      templatesHint:
        '模板要先在腾讯云申请并审核通过，可以参考下面的建议模板。变量的个数和顺序必须和建议模板一致，每个变量最多 20 个字，超出部分会被截掉。',
      columns: {
        send: '发送',
        templateId: '模板 ID'
      },
      templateIdPlaceholder: '模板 ID',
      suggested: '建议模板',
      params: '变量依次为：{params}',
      copy: '复制模板',
      copied: '模板已复制',
      always: '绑定时发送',
      kinds: {
        verify_code: {
          name: '验证码',
          hint: '用户在个人资料页绑定手机号时使用，不配置就不能绑定手机号。',
          params: '验证码、有效分钟数'
        },
        balance_low: {
          name: '余额不足提醒',
          hint: '用户余额降到提醒阈值以下时发送，开关和阈值沿用上面的「余额不足提醒」；每人每天最多一条。',
          params: '当前余额、提醒阈值'
        },
        subscription_expiry: {
          name: '订阅到期提醒',
          hint: '订阅到期前 7 天、3 天、1 天各发一条到用户绑定的手机号，和到期提醒邮件各自发送。',
          params: '订阅名称、到期时间、剩余天数'
        },
        ops_alert: {
          name: '运维告警',
          hint: '运维监控的告警规则触发时发给管理员手机号，遵守告警静默设置。',
          params: '规则名称、级别、当前值、阈值'
        },
        account_quota: {
          name: '账号限额告警',
          hint: '账号用量达到告警线时发给管理员手机号；需要打开上面的「账号限额通知」，并在账号里设置告警阈值。每个账号的每种限额每天最多一条。',
          params: '账号名称、限额类型、剩余额度、额度上限'
        }
      },
      adminPhones: '管理员告警手机号',
      adminPhonesHint: '运维告警和账号限额告警发到这些号码。每行一个，最多 10 个，仅支持中国大陆手机号。',
      minLevel: '运维告警最低级别',
      minLevels: {
        all: '全部级别',
        info: '提示（P2、P3）及以上',
        warning: '警告（P1）及以上',
        critical: '仅严重（P0）'
      },
      dailyLimit: '每天上限',
      dailyLimitHint: '每个用户每天最多收到的通知短信条数，管理员告警按每个手机号计；0 为不限。验证码和测试短信不计入。',
      dailyLimitUnit: '条',
      save: '保存短信设置',
      saving: '保存中…',
      saved: '短信设置已保存',
      saveFailed: '短信设置保存失败',
      unsaved: '有未保存的修改',
      loadFailed: '短信设置加载失败',
      retry: '重试',
      showLogs: '发送记录',
      test: {
        title: '发送测试短信',
        hint: '用已保存的设置，把所选模板填上示例内容发到这个号码；没打开「启用短信通知」也能测试。',
        phonePlaceholder: '接收测试短信的手机号',
        send: '发送测试',
        sending: '发送中…',
        saveFirst: '先保存设置再测试',
        sent: '测试短信已发出',
        dryRun: '已写入发送记录（仅记录不发送）',
        failed: '发送失败：{error}',
        requestFailed: '测试短信发送失败'
      },
      validation: {
        phones: '这些号码不是中国大陆手机号：{phones}',
        tooManyPhones: '管理员手机号最多 10 个',
        templateId: '模板 ID 是一串数字，例如 1234567',
        appId: 'SDK AppID 是一串数字，例如 1400000000',
        dailyLimit: '每天上限要在 0 到 1000 之间',
        credentials: '启用前要填好 SecretId、SecretKey、SDK AppID 和短信签名，或者打开「仅记录不发送」'
      },
      log: {
        title: '短信发送记录',
        hint: '最近 90 天的短信，最新的在前。验证码不会保存，仅记录不发送模式除外。',
        searchPlaceholder: '搜索手机号或用户邮箱',
        search: '搜索',
        empty: '还没有发送记录',
        noMatch: '没有匹配的记录',
        loadFailed: '发送记录加载失败',
        retry: '重试',
        admin: '管理员',
        columns: {
          time: '时间',
          event: '类型',
          phone: '手机号',
          user: '接收人',
          content: '模板与变量',
          status: '状态'
        },
        events: {
          verify_code: '验证码',
          test: '测试',
          balance_low: '余额不足',
          subscription_expiry: '订阅到期',
          ops_alert: '运维告警',
          account_quota: '账号限额'
        },
        statuses: {
          pending: '发送中',
          sent: '已发送',
          failed: '失败',
          dry_run: '仅记录',
          limited: '超出每天上限'
        }
      },
      user: {
        title: '短信提醒',
        description: '绑定手机号后，{events}会发短信到这个号码，和通知邮箱互不影响。只能绑定一个中国大陆手机号。',
        and: '和',
        events: {
          balance_low: '余额不足提醒',
          subscription_expiry: '订阅到期提醒'
        },
        phonePlaceholder: '手机号',
        sendCode: '获取验证码',
        resend: '重新获取',
        countdown: '{seconds} 秒后重新获取',
        codePlaceholder: '6 位验证码',
        bind: '验证并绑定',
        cancel: '取消',
        verified: '已验证',
        remove: '解绑',
        codeSent: '验证码已发送到 {phone}',
        bound: '手机号已绑定',
        removed: '手机号已解绑',
        paused: '已暂停短信提醒',
        resumed: '已恢复短信提醒',
        sendFailed: '验证码发送失败',
        bindFailed: '绑定失败',
        updateFailed: '操作失败',
        loadFailed: '短信提醒加载失败'
      },
      errors: {
        MOK_SMS_UNAVAILABLE: '短信提醒暂不可用',
        MOK_SMS_PHONE_INVALID: '请输入正确的中国大陆手机号',
        MOK_SMS_CODE_COOLDOWN: '获取太频繁，请 {seconds} 秒后再试',
        MOK_SMS_CODE_DAILY_LIMIT: '今天获取验证码的次数已用完，请明天再试',
        MOK_SMS_CODE_INVALID: '验证码不正确',
        MOK_SMS_CODE_EXPIRED: '验证码已失效，请重新获取',
        MOK_SMS_SEND_FAILED: '短信发送失败，请稍后再试',
        MOK_SMS_PHONE_NOT_BOUND: '还没有绑定手机号',
        MOK_SMS_NOT_CONFIGURED: '先填好并保存腾讯云配置和所选模板的 ID'
      }
    },
    paymentQuickAmounts: {
      title: '充值快捷金额',
      description:
        '充值页上可以直接点选的金额按钮。不设置时使用默认的 {defaults}。超出支付方式最低、最高金额的按钮仍会自动隐藏，充值优惠阶梯的价签照常显示在对应按钮上。',
      label: '快捷金额',
      placeholder: '例如：30, 50, 100, 200, 500',
      hint: '用逗号或空格分隔，最多 12 个，最多两位小数；保存后按从小到大排列。清空后保存即恢复默认。',
      preview: '预览',
      usingDefault: '（默认金额）',
      save: '保存',
      saving: '保存中…',
      saved: '快捷金额已保存',
      saveFailed: '快捷金额保存失败',
      reset: '恢复默认',
      resetDone: '已恢复默认快捷金额',
      unsaved: '有未保存的修改',
      loadFailed: '快捷金额加载失败',
      retry: '重试',
      errors: {
        invalid: '无法识别「{value}」，请填写数字',
        range: '「{value}」超出范围：金额要大于 0，且不超过 1000000',
        decimals: '「{value}」的小数超过两位',
        tooMany: '最多 {max} 个金额'
      }
    }
  }
}
