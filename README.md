# Moodctrl Backend

written in golang

Initializes a PWM chip using `sysfs` of the Linux Kernel.
It was written for the NXP PCA9685 but any other chip should work.

Depending on the platform something like

```
echo pca9685-pwm 0x60 > /sys/class/i2c-dev/i2c-1/device/new_device
```

is necessary to have the chip appear under the `/sys/class/pwm`
directory.

# Frontend

A frontend written in TypeScript can be found here:

https://github.com/mguentner/moodctrl-front

# License

GPLv3
