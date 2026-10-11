/**
 * Live in-app checkout is paused. The purchase page shows the card-shop
 * buy step plus on-page redeem instead of amount, payment method, and
 * create-order controls.
 *
 * Set this to false to restore the in-app recharge and subscription checkout.
 * An in-flight WeChat return still uses the existing payment status panel.
 */
export const ONLINE_PAYMENT_CHECKOUT_HIDDEN = true
