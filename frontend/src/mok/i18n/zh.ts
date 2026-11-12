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
        view: '查看明细（{count}）'
      }
    }
  }
}
