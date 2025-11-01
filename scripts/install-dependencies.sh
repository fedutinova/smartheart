#!/bin/bash

set -e

echo "🔍 Проверка зависимостей для SmartHeart..."

# Проверка операционной системы
if [ -f /etc/fedora-release ] || [ -f /etc/redhat-release ]; then
    echo "📦 Обнаружена Fedora/RHEL система"
    PKG_MGR="dnf"
    GCC_PKG="gcc-c++"
    OPENCV_PKG="opencv-devel"
    PKGCONFIG_PKG="pkgconfig"
elif [ -f /etc/debian_version ]; then
    echo "📦 Обнаружена Debian/Ubuntu система"
    PKG_MGR="apt-get"
    GCC_PKG="build-essential"
    OPENCV_PKG="libopencv-dev"
    PKGCONFIG_PKG="pkg-config"
elif [[ "$OSTYPE" == "darwin"* ]]; then
    echo "📦 Обнаружена macOS система"
    echo "Установите зависимости вручную:"
    echo "  brew install opencv pkg-config"
    exit 0
else
    echo "❌ Неизвестная система. Установите зависимости вручную."
    exit 1
fi

# Проверка наличия g++
if command -v g++ &> /dev/null; then
    echo "✅ g++ уже установлен: $(which g++)"
    GCC_NEEDED=false
else
    echo "❌ g++ не найден"
    GCC_NEEDED=true
fi

# Проверка OpenCV
if pkg-config --exists opencv4 2>/dev/null; then
    OPENCV_VERSION=$(pkg-config --modversion opencv4)
    echo "✅ OpenCV установлен: версия $OPENCV_VERSION"
    OPENCV_NEEDED=false
else
    echo "❌ OpenCV не найден"
    OPENCV_NEEDED=true
fi

# Проверка pkg-config
if command -v pkg-config &> /dev/null; then
    echo "✅ pkg-config уже установлен: $(which pkg-config)"
    PKGCONFIG_NEEDED=false
else
    echo "❌ pkg-config не найден"
    PKGCONFIG_NEEDED=true
fi

# Установка недостающих пакетов
PACKAGES_TO_INSTALL=""

if [ "$GCC_NEEDED" = true ]; then
    PACKAGES_TO_INSTALL="$PACKAGES_TO_INSTALL $GCC_PKG"
fi

if [ "$OPENCV_NEEDED" = true ]; then
    PACKAGES_TO_INSTALL="$PACKAGES_TO_INSTALL $OPENCV_PKG"
fi

if [ "$PKGCONFIG_NEEDED" = true ]; then
    PACKAGES_TO_INSTALL="$PACKAGES_TO_INSTALL $PKGCONFIG_PKG"
fi

if [ -n "$PACKAGES_TO_INSTALL" ]; then
    echo ""
    echo "📥 Требуется установить следующие пакеты:$PACKAGES_TO_INSTALL"
    echo ""
    
    if [ "$PKG_MGR" = "dnf" ]; then
        echo "Выполните команду:"
        echo "  sudo dnf install$PACKAGES_TO_INSTALL"
    elif [ "$PKG_MGR" = "apt-get" ]; then
        echo "Выполните команду:"
        echo "  sudo apt-get update && sudo apt-get install$PACKAGES_TO_INSTALL"
    fi
    
    exit 1
else
    echo ""
    echo "✅ Все зависимости установлены!"
    echo ""
    echo "Проверка Go окружения..."
    
    # Проверка CGO
    if [ "$CGO_ENABLED" != "1" ]; then
        echo "⚠️  CGO_ENABLED не установлен в 1"
        echo "   Рекомендуется выполнить: export CGO_ENABLED=1"
    else
        echo "✅ CGO_ENABLED=1"
    fi
    
    echo ""
    echo "🎉 Готово к запуску!"
fi

